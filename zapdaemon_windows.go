package zapm

import (
	"github.com/kardianos/service"
	"log"
)

type ZapDaemon struct{}

func (p *ZapDaemon) Start(s service.Service) error {
	// 启动服务时执行的操作
	go p.run()
	return nil
}

func (p *ZapDaemon) run() {
	log.Println("Starting Zap Daemon...")
	err := StartMonitorServer()
	if err != nil {
		log.Fatal(err)
		return
	}
}

func (p *ZapDaemon) Stop(s service.Service) error {
	// 停止服务时执行的操作
	log.Println("ZAPM Service stopped.")
	return nil
}
