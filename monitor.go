package zapm

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/shirou/gopsutil/v3/process"
)

// ProcessMonitor 进程监控器
type ProcessMonitor struct {
	service    *Service
	process    *process.Process
	startTime  time.Time
	logger     *Logger
	stopChan   chan struct{}
	stopped    bool
	statsMutex sync.Mutex
}

// NewProcessMonitor 创建新的进程监控器
func NewProcessMonitor(service *Service, logger *Logger) (*ProcessMonitor, error) {
	proc, err := process.NewProcess(int32(service.Pid))
	if err != nil {
		return nil, fmt.Errorf("无法创建进程监控器: %v", err)
	}

	return &ProcessMonitor{
		service:   service,
		process:   proc,
		startTime: time.Now(),
		logger:    logger,
		stopChan:  make(chan struct{}),
	}, nil
}

// Start 开始监控
func (m *ProcessMonitor) Start() {
	go m.monitor()
}

// Stop 停止监控
func (m *ProcessMonitor) Stop() {
	close(m.stopChan)
}

// monitor 监控进程状态
func (m *ProcessMonitor) monitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for !m.stopped {
		select {
		case <-m.stopChan:
			m.stopped = true
			return
		case <-ticker.C:
			if m.stopped {
				return
			}
			m.updateStats()
		}
	}
}

// updateStats 更新进程统计信息
func (m *ProcessMonitor) updateStats() {
	// 更新运行时间
	uptime := int64(time.Since(m.startTime).Seconds())

	// 获取 CPU 和内存使用情况
	var cpuPercent float64
	var memoryUsage int64
	var err error
	if runtime.GOOS == "windows" {
		// Windows 平台特殊处理
		m.logger.Debug("Windows平台: 开始获取进程资源使用情况 (PID: %d)", m.process.Pid)

		// 尝试多次获取 CPU 使用率
		for i := 0; i < 3; i++ {
			cpuPercent, err = m.process.CPUPercent()
			if err == nil && cpuPercent > 0 {
				m.logger.Debug("Windows平台: gopsutil获取CPU使用率成功: %.2f%%", cpuPercent)
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if err != nil || cpuPercent <= 0 {
			m.logger.Warn("Windows平台: gopsutil获取CPU使用率失败: %v", err)
			// 使用备用方法
			backupCPU := getWindowsCPUUsage(int(m.process.Pid))
			if backupCPU > 0 {
				m.logger.Debug("Windows平台: 备用方法获取CPU使用率成功: %.2f%%", backupCPU)
				cpuPercent = backupCPU
			} else {
				m.logger.Warn("Windows平台: 所有CPU使用率获取方法均失败")
			}
		}

		// 获取内存使用
		memInfo, err := m.process.MemoryInfo()
		if err != nil || memInfo == nil {
			m.logger.Warn("Windows平台: gopsutil获取内存使用失败: %v", err)
			// 使用备用方法
			backupMem := getWindowsMemoryUsage(int(m.process.Pid))
			if backupMem > 0 {
				m.logger.Debug("Windows平台: 备用方法获取内存使用成功: %s", FormatBytes(backupMem))
				memoryUsage = backupMem
			} else {
				m.logger.Warn("Windows平台: 所有内存使用获取方法均失败")
			}
		} else {
			memoryUsage = int64(memInfo.RSS)
			m.logger.Debug("Windows平台: gopsutil获取内存使用成功: %s", FormatBytes(memoryUsage))
		}
	} else {
		// Linux/Unix 平台
		cpuPercent, err = m.process.CPUPercent()
		if err != nil {
			m.logger.Warn("获取CPU使用率失败: %v", err)
			cpuPercent = 0
		}

		memInfo, err := m.process.MemoryInfo()
		if err != nil {
			m.logger.Warn("获取内存使用失败: %v", err)
			memoryUsage = 0
		} else {
			memoryUsage = int64(memInfo.RSS)
		}
	}

	// 线程安全地更新服务状态
	m.statsMutex.Lock()
	m.service.Uptime = uptime
	m.service.CpuUsage = cpuPercent
	m.service.MemoryUsage = memoryUsage
	m.statsMutex.Unlock()

	// 记录调试信息
	m.logger.Debug("更新服务状态 - 名称: %s, PID: %d, CPU: %.2f%%, 内存: %s, 运行时间: %ds",
		m.service.Name,
		m.service.Pid,
		cpuPercent,
		FormatBytes(memoryUsage),
		uptime)

	// 记录资源警告
	if cpuPercent > 80 {
		m.logger.Warn("CPU 使用率过高: %.2f%%", cpuPercent)
	}
	if memoryUsage > 1024*1024*1024 {
		m.logger.Warn("内存使用过高: %d MB", memoryUsage/1024/1024)
	}

	// 跨平台进程存活性检查
	if runtime.GOOS == "windows" {
		// Windows平台检查方式
		proc, err := os.FindProcess(int(m.process.Pid))
		if err != nil {
			m.logger.Error("查找进程失败: %v", err)
			return
		}
		// 发送0信号检查进程是否存活
		err = proc.Signal(syscall.Signal(0))
		if err != nil {
			m.logger.Error("进程已停止运行: %v", err)
			return
		}
	} else {
		// Linux/Unix平台检查方式
		_, err := m.process.Cmdline()
		if err != nil {
			m.logger.Error("进程已停止运行: %v", err)
			return
		}
		// 额外检查进程状态
		status, err := m.process.Status()
		if err != nil {
			m.logger.Error("获取进程状态失败: %v", err)
			return
		}

		if stringInSlice(status, "zombie") {
			m.logger.Error("进程状态异常: %s", status)
			return
		}
	}

	// 在 Windows 上获取额外的性能计数器
	if runtime.GOOS == "windows" {
		// 获取句柄数
		handles, err := m.process.NumThreads()
		if err == nil && handles > 1000 {
			m.logger.Warn("线程数过多: %d", handles)
		}
	}
}

// GetStats 获取进程统计信息
func (m *ProcessMonitor) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"pid":          m.service.Pid,
		"name":         m.service.Name,
		"status":       m.service.Status,
		"uptime":       m.service.Uptime,
		"cpu_usage":    m.service.CpuUsage,
		"memory_usage": m.service.MemoryUsage,
		"restarts":     m.service.RestartCount,
		"last_restart": m.service.LastRestart,
	}

	// 获取进程命令行
	cmdline, err := m.process.Cmdline()
	if err == nil {
		stats["cmdline"] = cmdline
	}

	// 获取进程工作目录
	cwd, err := m.process.Cwd()
	if err == nil {
		stats["cwd"] = cwd
	}

	// 获取进程创建时间
	createTime, err := m.process.CreateTime()
	if err == nil {
		stats["create_time"] = time.Unix(createTime/1000, 0).Format(time.RFC3339)
	}

	// 获取进程用户
	username, err := m.process.Username()
	if err == nil {
		stats["username"] = username
	}

	return stats
}

// FormatUptime 格式化运行时间
func FormatUptime(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	secs := int(duration.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, secs)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

// stringInSlice 检查字符串是否在切片中
func stringInSlice(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Windows平台特定的CPU使用率获取函数
func getWindowsCPUUsage(pid int) float64 {
	// 方法1: 使用WMI查询
	cmd := exec.Command("wmic", "path", "Win32_PerfFormattedData_PerfProc_Process",
		"where", fmt.Sprintf("IDProcess=%d", pid), "get", "PercentProcessorTime")

	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 {
			cpuStr := strings.TrimSpace(lines[1])
			if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
				return cpu
			}
		}
	}

	// 方法2: 使用PowerShell的Get-Counter
	cmd = exec.Command("powershell", "-Command", fmt.Sprintf(`
		try {
			$counter = Get-Counter "\\Process(*)\\%% Processor Time" -ErrorAction Stop
			$sample = $counter.CounterSamples | Where-Object { $_.InstanceName -eq "%d" }
			if ($sample) { $sample.CookedValue }
		} catch { }
	`, pid))

	output, err = cmd.Output()
	if err == nil {
		if cpuStr := strings.TrimSpace(string(output)); cpuStr != "" {
			if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
				return cpu
			}
		}
	}

	// 方法3: 使用Get-Process计算
	cmd = exec.Command("powershell", "-Command", fmt.Sprintf(`
		$process = Get-Process -Id %d -ErrorAction SilentlyContinue
		if ($process) {
			$cpuTime = ($process.TotalProcessorTime - $process.UserProcessorTime).TotalMilliseconds
			$uptime = (Get-Date) - $process.StartTime
			if ($uptime.TotalMilliseconds -gt 0) {
				($cpuTime / $uptime.TotalMilliseconds) * 100
			}
		}
	`, pid))

	output, err = cmd.Output()
	if err == nil {
		if cpuStr := strings.TrimSpace(string(output)); cpuStr != "" {
			if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
				return cpu
			}
		}
	}

	return 0
}

// Windows平台特定的内存使用获取函数
func getWindowsMemoryUsage(pid int) int64 {
	// 方法1: 使用WMI查询
	cmd := exec.Command("wmic", "path", "Win32_PerfFormattedData_PerfProc_Process",
		"where", fmt.Sprintf("IDProcess=%d", pid), "get", "WorkingSetPrivate")

	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 {
			memStr := strings.TrimSpace(lines[1])
			if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
				return mem
			}
		}
	}

	// 方法2: 使用PowerShell的Get-Process
	cmd = exec.Command("powershell", "-Command", fmt.Sprintf(`
		$process = Get-Process -Id %d -ErrorAction SilentlyContinue
		if ($process) {
			$process.WorkingSet64
		}
	`, pid))

	output, err = cmd.Output()
	if err == nil {
		if memStr := strings.TrimSpace(string(output)); memStr != "" {
			if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
				return mem
			}
		}
	}

	// 方法3: 使用tasklist命令
	cmd = exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 {
			fields := strings.Split(strings.Trim(lines[0], `"`), `","`)
			if len(fields) >= 5 {
				// 内存字段通常是第5个字段，格式类似 "8,192 K"
				memStr := strings.Trim(fields[4], `"`)
				memStr = strings.Replace(memStr, ",", "", -1)
				memStr = strings.Replace(memStr, " K", "", -1)
				if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
					return mem * 1024 // 转换为字节
				}
			}
		}
	}

	return 0
}

// FormatBytes 格式化字节数
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// SystemMonitor 系统资源监控器
type SystemMonitor struct {
	cpuUsage float64
	memUsage float64
	mutex    sync.Mutex
	stopChan chan struct{}
	stopped  bool
}

// NewSystemMonitor 创建新的系统资源监控器
func NewSystemMonitor() *SystemMonitor {
	return &SystemMonitor{
		stopChan: make(chan struct{}),
	}
}

// Start 开始系统资源监控
func (m *SystemMonitor) Start() {
	go m.monitor()
}

// Stop 停止系统资源监控
func (m *SystemMonitor) Stop() {
	close(m.stopChan)
}

// monitor 监控系统资源使用情况
func (m *SystemMonitor) monitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for !m.stopped {
		select {
		case <-m.stopChan:
			m.stopped = true
			return
		case <-ticker.C:
			if m.stopped {
				return
			}
			m.updateSystemStats()
		}
	}
}

// updateSystemStats 更新系统资源统计信息
func (m *SystemMonitor) updateSystemStats() {
	// 获取系统CPU使用率
	cpuPercents, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercents) > 0 {
		m.mutex.Lock()
		m.cpuUsage = cpuPercents[0]
		m.mutex.Unlock()
	}

	// 获取系统内存使用率
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		m.mutex.Lock()
		m.memUsage = memInfo.UsedPercent
		m.mutex.Unlock()
	}
}

// GetSystemStats 获取系统资源统计信息
func (m *SystemMonitor) GetSystemStats() (float64, float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.cpuUsage, m.memUsage
}
