package commands

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/zapj/zapm"
)

var startCmd = &cobra.Command{
	Use:   "start [service name]",
	Short: "启动指定的服务",
	Long:  `启动配置文件中定义的指定服务，例如: zapm start myservice`,
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

		// 检查是否使用HTTP API启动
		if zapm.Conf.Server.Address != "" {
			apiUrl := fmt.Sprintf("http://%s:%d/api/service/%s/start", zapm.Conf.Server.Address, zapm.Conf.Server.Port, serviceName)
			fmt.Printf("通过API启动服务: %s\n", apiUrl)

			// 发送HTTP请求
			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("POST", apiUrl, nil)
			if err != nil {
				fmt.Printf("创建API请求失败: %v\n", err)
				os.Exit(1)
			}

			// 重试最多3次
			var resp *http.Response
			for i := 0; i < 3; i++ {
				resp, err = client.Do(req)
				if err == nil {
					break
				}
				time.Sleep(1 * time.Second)
			}

			if err != nil {
				fmt.Printf("API请求失败: %v\n", err)
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("API返回错误状态码: %d\n", resp.StatusCode)
				os.Exit(1)
			}

			fmt.Printf("API请求成功，服务已启动\n")
		}

	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
