package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zapj/zapm"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all services",
	Run: func(cmd *cobra.Command, args []string) {

		rs, err := zapm.HttpGetRequest(zapm.ServerPath("/show_services"))
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(rs)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
