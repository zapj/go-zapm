package zapm

import (
	"fmt"
)

type ZapmConfig struct {
	Server   ServerConfig `yaml:"server"`
	Services []Service    `yaml:"services"`
}

type ServerConfig struct {
	Address string `yaml:"address" json:"address"`
	Port    int    `yaml:"port" json:"port"`
}

type Service struct {
	Name        string `yaml:"name" json:"name"`
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`

	// 启动类型 auto   manual
	StartupType string            `yaml:"startup_type" json:"startupType"`
	Run         string            `yaml:"run" json:"run"`
	Cron        string            `yaml:"cron" json:"cron"`
	Status      string            `yaml:"status" json:"status"`
	Args        []string          `yaml:"args" json:"args"`
	WorkDir     string            `yaml:"work_dir" json:"workDir"`
	Env         map[string]string `yaml:"env" json:"env"`
	PidFile     string            `yaml:"pid_file" json:"pidFile"`
	Pid         int               `yaml:"pid" json:"pid"`
	StartTime   string            `yaml:"start_time" json:"startTime"`

	// 自动重启配置
	AutoRestart bool `yaml:"auto_restart" json:"autoRestart"`
	MaxRetries  int  `yaml:"max_retries" json:"maxRetries"`
	RetryDelay  int  `yaml:"retry_delay" json:"retryDelay"` // 重试延迟，单位秒

	// 日志配置
	LogFile     string `yaml:"log_file" json:"logFile"`
	LogMaxSize  int    `yaml:"log_max_size" json:"logMaxSize"`   // 日志文件最大大小，单位MB
	LogMaxFiles int    `yaml:"log_max_files" json:"logMaxFiles"` // 保留的日志文件数量
	LogLevel    string `yaml:"log_level" json:"logLevel"`        // 日志级别：DEBUG, INFO, WARN, ERROR, FATAL
	LogCompress bool   `yaml:"log_compress" json:"logCompress"`  // 是否压缩旧日志

	// 统计信息
	RestartCount int    `yaml:"restart_count" json:"restartCount"` // 重启次数
	Uptime       int64  `yaml:"uptime" json:"uptime"`              // 运行时间（秒）
	LastRestart  string `yaml:"last_restart" json:"lastRestart"`   // 最后一次重启时间

	// 资源使用
	CpuUsage    float64 `yaml:"cpu_usage" json:"cpuUsage"`       // CPU使用率（百分比）
	MemoryUsage int64   `yaml:"memory_usage" json:"memoryUsage"` // 内存使用（字节）
}

func NewDefaultZapmConfig() *ZapmConfig {
	return &ZapmConfig{
		Server: ServerConfig{
			Address: "127.0.0.1",
			Port:    2428,
		},
	}
}

func ServerPath(path string) string {
	return fmt.Sprintf("http://%s:%d%s", Conf.Server.Address, Conf.Server.Port, path)
}
