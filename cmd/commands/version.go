package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zapj/zapm"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of zapm",
	Long:  `zapm is a services management.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("zapm %s - %s\r\n", zapm.Version, zapm.BuildDate)
	},
}
