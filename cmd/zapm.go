package main

import (
	"os"
	"path/filepath"

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
	} else {
		// 使用程序所在目录
		execDir := filepath.Dir(execPath)
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

	// 设置日志目录
	var logsDir string
	if execPath != "" {
		// 如果成功获取了可执行文件路径，使用其所在目录
		logsDir = filepath.Join(filepath.Dir(execPath), "logs")
	} else {
		// 否则使用当前工作目录
		currentDir, err := os.Getwd()
		if err != nil {
			panic("获取当前工作目录失败: " + err.Error())
		}
		logsDir = filepath.Join(currentDir, "logs")
	}

	// 获取日志目录的绝对路径
	absLogsDir, err := filepath.Abs(logsDir)
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
