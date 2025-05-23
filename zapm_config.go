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
	Name        string `yaml:"name"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`

	// 启动类型 Auto   manual
	StartupType string   `yaml:"startup_type"`
	Run         string   `yaml:"run"`
	Cron        string   `yaml:"cron"`
	Status      string   `yaml:"status"`
	Args        []string `yaml:"args"`
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
