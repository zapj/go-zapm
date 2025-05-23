package zapm

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kardianos/service"
)

type ZapDaemon struct {
	stopChan chan struct{}
	loggers  map[string]*Logger         // 服务日志管理器映射
	monitors map[string]*ProcessMonitor // 进程监控器映射
	mu       sync.Mutex                 // 保护映射
}

func NewZapDaemon() *ZapDaemon {
	return &ZapDaemon{
		stopChan: make(chan struct{}),
		loggers:  make(map[string]*Logger),
		monitors: make(map[string]*ProcessMonitor),
	}
}

// getLogger 获取或创建服务的日志管理器
func (p *ZapDaemon) getLogger(service *Service) (*Logger, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果已存在日志管理器，直接返回
	if logger, ok := p.loggers[service.Name]; ok {
		return logger, nil
	}

	// 确定日志文件路径
	logPath := service.LogFile
	if logPath == "" {
		logPath = filepath.Join("logs", fmt.Sprintf("%s.log", service.Name))
	}

	// 创建新的日志管理器
	logger, err := NewLogger(
		logPath,
		service.LogMaxSize,
		service.LogMaxFiles,
		service.LogLevel,
		service.LogCompress,
		service.Name,
	)
	if err != nil {
		return nil, fmt.Errorf("创建日志管理器失败: %v", err)
	}

	p.loggers[service.Name] = logger
	return logger, nil
}

func (p *ZapDaemon) Start(s service.Service) error {
	// 启动服务时执行的操作
	go p.run()
	return nil
}

func (p *ZapDaemon) run() {
	log.Println("Starting Zap Daemon...")

	// 启动监控服务器
	err := StartMonitorServer()
	if err != nil {
		log.Fatal(err)
		return
	}

	// 启动进程监控
	go p.startProcessMonitor()

	// 启动自动运行的程序
	go p.startAutoRunner()

	// 等待停止信号
	<-p.stopChan
}

// 监控所有服务进程的状态
func (p *ZapDaemon) startProcessMonitor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			for i := range Conf.Services {
				service := &Conf.Services[i]

				// 只监控已启动的服务
				if service.Status != "running" || service.Pid == 0 {
					continue
				}

				// 检查进程是否存在
				process, err := os.FindProcess(service.Pid)
				if err != nil {
					p.handleProcessDeath(service)
					continue
				}

				// 在 Windows 上，FindProcess 总是成功，需要额外的信号测试
				err = process.Signal(syscall.Signal(0))
				if err != nil {
					p.handleProcessDeath(service)
					continue
				}
			}
		}
	}
}

// 处理进程意外终止
func (p *ZapDaemon) handleProcessDeath(service *Service) {
	// 获取日志管理器
	logger, _ := p.getLogger(service)

	// 停止监控器
	p.mu.Lock()
	if monitor, ok := p.monitors[service.Name]; ok {
		monitor.Stop()
		delete(p.monitors, service.Name)
	}
	p.mu.Unlock()

	// 如果启用了自动重启
	if service.AutoRestart {
		if logger != nil {
			logger.Warn("服务已终止，准备重启")
		}
		log.Printf("服务 '%s' (PID: %d) 已终止，准备重启\n", service.Name, service.Pid)

		// 更新重启统计信息
		service.RestartCount++
		service.LastRestart = time.Now().Format(time.RFC3339)

		// 重置服务状态
		service.Status = "stopped"
		service.Pid = 0

		// 等待指定的延迟时间
		if service.RetryDelay > 0 {
			time.Sleep(time.Duration(service.RetryDelay) * time.Second)
		}

		// 重启服务
		if err := p.startService(service); err != nil {
			if logger != nil {
				logger.Error("重启失败: %v", err)
			}
			log.Printf("重启服务 '%s' 失败: %v\n", service.Name, err)
		}
	} else {
		if logger != nil {
			logger.Info("服务已终止")
		}
		log.Printf("服务 '%s' (PID: %d) 已终止\n", service.Name, service.Pid)
		service.Status = "stopped"
		service.Pid = 0
	}
}

// 启动自动运行的服务
func (p *ZapDaemon) startAutoRunner() {
	// 等待系统完全启动
	time.Sleep(5 * time.Second)

	for i := range Conf.Services {
		service := &Conf.Services[i]
		if service.StartupType == "auto" {
			log.Printf("自动启动服务 '%s'\n", service.Name)
			if err := p.startService(service); err != nil {
				log.Printf("启动服务 '%s' 失败: %v\n", service.Name, err)
			}
		}
	}
}

// 启动单个服务
func (p *ZapDaemon) startService(service *Service) error {
	cmdParts := strings.Fields(service.Run)
	if len(cmdParts) == 0 {
		return fmt.Errorf("服务 '%s' 的命令无效", service.Name)
	}

	command := exec.Command(cmdParts[0], cmdParts[1:]...)

	if service.WorkDir != "" {
		command.Dir = service.WorkDir
	}

	if service.Env != nil && len(service.Env) > 0 {
		env := os.Environ()
		for key, value := range service.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		command.Env = env
	}

	// 获取或创建日志管理器
	logger, err := p.getLogger(service)
	if err != nil {
		return err
	}

	// 记录启动信息
	logger.Info("正在启动服务...")

	// 设置命令输出到日志管理器
	command.Stdout = logger
	command.Stderr = logger

	// 启动进程
	if err := command.Start(); err != nil {
		logger.Error("启动失败: %v", err)
		return fmt.Errorf("启动失败: %v", err)
	}

	// 更新服务状态
	service.Status = "running"
	service.Pid = command.Process.Pid

	// 记录启动时间
	now := time.Now()
	service.StartTime = now.Format(time.RFC3339)

	// 记录启动成功信息
	logger.Info("服务已启动，PID: %d", service.Pid)
	log.Printf("服务 '%s' 已启动，PID: %d\n", service.Name, service.Pid)

	// 创建并启动进程监控器
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果已存在监控器，先停止它
	if monitor, ok := p.monitors[service.Name]; ok {
		monitor.Stop()
	}

	// 创建新的监控器
	monitor, err := NewProcessMonitor(service, logger)
	if err != nil {
		logger.Warn("无法创建进程监控器: %v", err)
	} else {
		p.monitors[service.Name] = monitor
		monitor.Start()
		logger.Info("进程监控已启动")
	}

	return nil
}

func (p *ZapDaemon) Stop(s service.Service) error {
	// 停止服务时执行的操作
	log.Println("正在停止 ZAPM 服务...")

	// 发送停止信号
	close(p.stopChan)

	// 停止所有监控器
	p.mu.Lock()
	for _, monitor := range p.monitors {
		monitor.Stop()
	}
	p.monitors = make(map[string]*ProcessMonitor)
	p.mu.Unlock()

	// 停止所有运行中的服务
	for i := range Conf.Services {
		service := &Conf.Services[i]
		if service.Status == "running" && service.Pid > 0 {
			// 获取日志管理器
			logger, _ := p.getLogger(service)

			if process, err := os.FindProcess(service.Pid); err == nil {
				log.Printf("正在停止服务 '%s' (PID: %d)\n", service.Name, service.Pid)
				if logger != nil {
					logger.Info("正在停止服务")
				}

				_ = process.Signal(syscall.SIGTERM)

				// 等待进程退出
				time.Sleep(3 * time.Second)

				// 如果进程仍在运行，强制终止
				if err := process.Signal(syscall.Signal(0)); err == nil {
					if logger != nil {
						logger.Warn("服务未响应 SIGTERM，正在强制终止")
					}
					_ = process.Kill()
				}
			}

			service.Status = "stopped"
			service.Pid = 0
		}
	}

	// 关闭所有日志管理器
	for _, logger := range p.loggers {
		logger.Close()
	}
	p.loggers = make(map[string]*Logger)

	log.Println("ZAPM 服务已停止")
	return nil
}
