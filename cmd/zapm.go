package main

import (
	"github.com/zapj/zapm"
	"github.com/zapj/zapm/cmd/commands"
	"gopkg.in/yaml.v3"
	"os"
)

var (
	Version   string = "v1.0.1"
	BuildDate string
)

func main() {
	zapm.Version = Version
	zapm.BuildDate = BuildDate

	zapmConfigBytes, err := os.ReadFile("zapm.yml")
	if err == nil {
		err = yaml.Unmarshal(zapmConfigBytes, zapm.Conf)
	}

	commands.Execute()
}
