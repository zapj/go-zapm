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
	Long:  `All software has versions. This is zapm`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("zapm %s - %s", zapm.Version, zapm.BuildDate)
	},
}
