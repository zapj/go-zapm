package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

		// 解析命令和参数
		cmdParts := strings.Fields(service.Run)
		if len(cmdParts) == 0 {
			fmt.Printf("服务 '%s' 的命令无效\n", serviceName)
			os.Exit(1)
		}

		// 创建命令
		command := exec.Command(cmdParts[0], cmdParts[1:]...)

		// 设置工作目录（如果指定）
		if service.WorkDir != "" {
			command.Dir = service.WorkDir
		}

		// 设置环境变量
		if service.Env != nil && len(service.Env) > 0 {
			// 获取当前环境变量
			env := os.Environ()

			// 添加服务配置的环境变量
			for key, value := range service.Env {
				env = append(env, fmt.Sprintf("%s=%s", key, value))
			}

			command.Env = env
		}

		// 创建日志目录
		logDir := "logs"
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("创建日志目录失败: %v\n", err)
			os.Exit(1)
		}

		// 确定日志文件路径
		var logFilePath string
		if service.LogFile != "" {
			// 如果指定了日志文件，使用指定的路径
			logFilePath = service.LogFile
			// 确保日志文件所在目录存在
			logFileDir := filepath.Dir(logFilePath)
			if err := os.MkdirAll(logFileDir, 0755); err != nil {
				fmt.Printf("创建日志目录失败: %v\n", err)
				os.Exit(1)
			}
		} else {
			// 否则使用默认路径
			logFilePath = filepath.Join(logDir, fmt.Sprintf("%s.log", serviceName))
		}

		// 打开日志文件
		f, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			fmt.Printf("打开日志文件失败: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		// 设置输出到日志文件
		command.Stdout = f
		command.Stderr = f

		// 启动进程
		if err := command.Start(); err != nil {
			fmt.Printf("启动服务 '%s' 失败: %v\n", serviceName, err)
			os.Exit(1)
		}

		// 更新服务状态
		service.Status = "running"
		service.Pid = command.Process.Pid
		service.StartTime = time.Now().Format(time.RFC3339)

		fmt.Printf("服务 '%s' 已启动，PID: %d\n", serviceName, service.Pid)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
