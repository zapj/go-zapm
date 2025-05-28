package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/zapj/zapm"
	"github.com/zapj/zapm/cmd/commands"
	"gopkg.in/yaml.v3"
)

var (
	Version   string = "v1.0.1"
	BuildDate string
)

func main() {
	zapm.Version = Version
	zapm.BuildDate = BuildDate

	// 获取程序所在目录
	var configPath string
	execPath, err := os.Executable()
	if err != nil {
		// 如果获取失败，使用当前目录
		configPath = "zapm.yml"
		zapm.LogsDir = "logs"
	} else if strings.Contains(execPath, "go-build") {
		// 如果是Go编译的临时目录，使用当前目录
		configPath = "zapm.yml"
		zapm.LogsDir = "logs"
	} else {
		// 使用程序所在目录
		execDir := filepath.Dir(execPath)
		zapm.LogsDir = filepath.Join(execDir, "logs")
		configPath = filepath.Join(execDir, "zapm.yml")
	}

	// 读取配置文件
	zapmConfigBytes, err := os.ReadFile(configPath)
	if err != nil {
		panic("无法读取配置文件 " + configPath + ": " + err.Error())
	}
	if err = yaml.Unmarshal(zapmConfigBytes, zapm.Conf); err != nil {
		panic("解析配置文件失败: " + err.Error())
	}

	// 获取日志目录的绝对路径
	absLogsDir, err := filepath.Abs(zapm.LogsDir)
	if err != nil {
		panic("获取日志目录绝对路径失败: " + err.Error())
	}

	// 创建日志目录
	if err := os.MkdirAll(absLogsDir, 0755); err != nil {
		panic("创建日志目录失败: " + err.Error())
	}

	// 设置全局日志目录路径
	zapm.LogsDir = absLogsDir

	commands.Execute()
}
