package commands

import "github.com/spf13/cobra"

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start Zap Monitor Server",
	Long:  `start Zap Monitor Server`,
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
