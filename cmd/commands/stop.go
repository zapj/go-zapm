package commands

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/zapj/zapm"
)

var stopCmd = &cobra.Command{
	Use:   "stop [service name]",
	Short: "停止指定的服务",
	Long:  `停止配置文件中定义的指定服务，例如: zapm stop myservice`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		serviceName := args[0]

		// 在配置中查找服务
		var service *zapm.Service
		for i := range zapm.Conf.Services {
			if zapm.Conf.Services[i].Name == serviceName {
				service = &zapm.Conf.Services[i]
				break
			}
		}

		if service == nil {
			fmt.Printf("服务 '%s' 在配置中未找到\n", serviceName)
			os.Exit(1)
		}

		if service.Pid == 0 {
			fmt.Printf("服务 '%s' 未在运行\n", serviceName)
			return
		}

		// 获取进程
		process, err := os.FindProcess(service.Pid)
		if err != nil {
			fmt.Printf("查找进程失败: %v\n", err)
			os.Exit(1)
		}

		// 发送终止信号
		err = process.Signal(syscall.SIGTERM)
		if err != nil {
			fmt.Printf("发送终止信号失败: %v\n", err)

			// 如果发送 SIGTERM 失败，尝试强制终止
			err = process.Kill()
			if err != nil {
				fmt.Printf("强制终止进程失败: %v\n", err)
				os.Exit(1)
			}
		}

		// 更新服务状态
		service.Status = "stopped"
		service.Pid = 0

		fmt.Printf("服务 '%s' 已停止\n", serviceName)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
