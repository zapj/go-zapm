package commands

import "github.com/spf13/cobra"

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop Zap Monitor Server",
	Long:  `stop Zap Monitor Server`,
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
