package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zapj/zapm"
)

var restartCmd = &cobra.Command{
	Use:   "restart [service name]",
	Short: "重启指定的服务",
	Long:  `重启配置文件中定义的指定服务，例如: zapm restart myservice`,
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

		// 先停止服务
		stopCmd.Run(cmd, args)

		// 然后启动服务
		startCmd.Run(cmd, args)

		fmt.Printf("服务 '%s' 已重启\n", serviceName)
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
