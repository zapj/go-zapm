package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zapj/zapm"
)

// Service 结构体用于解析服务信息
type Service struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	Pid         int     `json:"pid"`
	CpuUsage    float64 `json:"cpuUsage"`
	MemoryUsage float64 `json:"memoryUsage"`
	Uptime      int64   `json:"uptime"`
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all services",
	Run: func(cmd *cobra.Command, args []string) {
		// 获取服务列表
		rs, err := zapm.HttpGetRequest(zapm.ServerPath("/api/services"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}

		// 解析JSON响应
		var services []Service
		if err := json.Unmarshal([]byte(rs), &services); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
			return
		}

		// 创建表格writer
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		// 打印表头
		fmt.Fprintln(w, "NAME\tSTATUS\tPID\tCPU(%)\tMEM(MB)\tUPTIME")

		// 打印每个服务的信息
		for _, s := range services {
			// 格式化运行时间
			uptime := formatUptime(s.Uptime)

			// 格式化内存使用量（转换为MB）
			memMB := s.MemoryUsage / 1024 / 1024

			fmt.Fprintf(w, "%s\t%s\t%d\t%.1f\t%.1f\t%s\n",
				s.Name,
				s.Status,
				s.Pid,
				s.CpuUsage,
				memMB,
				uptime,
			)
		}

		// 刷新确保所有内容都被写入
		w.Flush()
	},
}

// formatUptime 将秒数转换为可读的时间格式
func formatUptime(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	} else if seconds < 86400 {
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		days := seconds / 86400
		hours := (seconds % 86400) / 3600
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
}
