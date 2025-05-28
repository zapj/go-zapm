package zapm

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/kardianos/service"
)

type ZapDaemon struct {
	stopChan chan struct{}
	loggers  map[string]*Logger // 服务日志管理器映射
	mu       sync.Mutex         // 保护映射
}

func NewZapDaemon() *ZapDaemon {
	return &ZapDaemon{
		stopChan: make(chan struct{}),
		loggers:  make(map[string]*Logger),
	}
}

// getLogger 获取或创建服务的日志管理器
func (p *ZapDaemon) getLogger(service *Service) (*Logger, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 确保loggers映射已初始化
	if p.loggers == nil {
		p.loggers = make(map[string]*Logger)
	}

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
	if service.Monitor != nil {
		service.Monitor.Stop()
		service.Monitor = nil
	}

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
		if err := startService(service); err != nil {
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
	log.Printf("正在自动启动服务...\n")
	for i := range Conf.Services {
		service := &Conf.Services[i]
		if service.StartupType == "auto" {
			log.Printf("自动启动服务 '%s'\n", service.Name)
			if err := startService(service); err != nil {
				log.Printf("启动服务 '%s' 失败: %v\n", service.Name, err)
			}
		}
	}
}

func (p *ZapDaemon) Stop(s service.Service) error {
	// 停止服务时执行的操作
	log.Println("正在停止 ZAPM 服务...")

	// 发送停止信号
	if p.stopChan != nil { // 添加 nil 检查
		close(p.stopChan)
	}

	// 停止所有服务的监控器
	for i := range Conf.Services {
		service := &Conf.Services[i]
		if service.Monitor != nil {
			service.Monitor.Stop()
			service.Monitor = nil
		}
	}

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
	if p.loggers != nil {
		for _, logger := range p.loggers {
			logger.Close()
		}
	}
	p.loggers = make(map[string]*Logger)

	log.Println("ZAPM 服务已停止")
	return nil
}
