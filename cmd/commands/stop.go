package commands

import (
	"fmt"
	"net/http"
	"os"

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

		// 调用API停止服务
		apiUrl := fmt.Sprintf("http://%s:%d/api/service/%s/stop", zapm.Conf.Server.Address, zapm.Conf.Server.Port, serviceName)
		resp, err := http.Post(apiUrl, "application/json", nil)
		if err != nil {
			fmt.Printf("调用停止API失败: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("停止服务失败: %s\n", resp.Status)
			os.Exit(1)
		}

		fmt.Printf("服务 '%s' 已停止\n", serviceName)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
