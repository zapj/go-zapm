package commands

import (
	"fmt"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"github.com/zapj/zapm"
	"log"
	"os"
)

func init() {
	rootCmd.AddCommand(serviceCmd)
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Windows platform service",
	Long:  `windows平台服务器安装与注册`,
	Run:   serviceExecute,
}

func serviceExecute(cmd *cobra.Command, args []string) {
	svcConfig := &service.Config{
		Name:        "zapm",
		DisplayName: "Zap Process Manager",
		Description: "Zapm is a process management tool.",
	}

	prg := &zapm.ZapDaemon{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 2 {
		cmd := os.Args[2]
		switch cmd {
		case "install":
			err = s.Install()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("服务安装成功")
			return
		case "uninstall":
			err = s.Uninstall()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("服务卸载成功")
			return
		}
	}
	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}
}
