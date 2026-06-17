package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isYaoNoistu/dbbackupctl/internal/index"
	"github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "查询备份记录",
		Long: `从本地索引查询备份记录。

子命令：
  mysql       查询 MySQL 备份记录
  postgresql  查询 PostgreSQL 备份记录
  all         查询全部备份记录

备份记录默认存储在 /data/dbbackupctl/index/backup_records.jsonl`,
		Example: `  dbbackupctl show mysql
  dbbackupctl show mysql --last 10
  dbbackupctl show mysql --job dev
  dbbackupctl show postgresql
  dbbackupctl show postgresql --last 10
  dbbackupctl show all
  dbbackupctl show mysql --json`,
	}

	cmd.AddCommand(
		newShowMySQLCmd(),
		newShowPostgreSQLCmd(),
		newShowAllCmd(),
	)

	return cmd
}

func newShowMySQLCmd() *cobra.Command {
	var (
		last int
		job  string
		json bool
	)

	cmd := &cobra.Command{
		Use:   "mysql",
		Short: "查询 MySQL 备份记录",
		Long: `从本地索引查询 MySQL 备份记录。

默认以表格显示最近 5 条记录。
使用 --last 调整显示数量。
使用 --json 输出机器可读格式。`,
		Example: `  dbbackupctl show mysql
  dbbackupctl show mysql --last 10
  dbbackupctl show mysql --job dev
  dbbackupctl show mysql --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow("mysql", last, job, json)
		},
	}

	cmd.Flags().IntVar(&last, "last", 5, "显示记录数量")
	cmd.Flags().StringVar(&job, "job", "", "按环境名过滤，例如 dev、prod")
	cmd.Flags().BoolVar(&json, "json", false, "以 JSON 格式输出")

	return cmd
}

func newShowPostgreSQLCmd() *cobra.Command {
	var (
		last int
		job  string
		json bool
	)

	cmd := &cobra.Command{
		Use:   "postgresql",
		Short: "查询 PostgreSQL 备份记录",
		Long: `从本地索引查询 PostgreSQL 备份记录。

默认以表格显示最近 5 条记录。
使用 --last 调整显示数量。
使用 --json 输出机器可读格式。`,
		Example: `  dbbackupctl show postgresql
  dbbackupctl show postgresql --last 10
  dbbackupctl show postgresql --job prod
  dbbackupctl show postgresql --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow("postgresql", last, job, json)
		},
	}

	cmd.Flags().IntVar(&last, "last", 5, "显示记录数量")
	cmd.Flags().StringVar(&job, "job", "", "按环境名过滤，例如 dev、prod")
	cmd.Flags().BoolVar(&json, "json", false, "以 JSON 格式输出")

	return cmd
}

func newShowAllCmd() *cobra.Command {
	var (
		last int
		json bool
	)

	cmd := &cobra.Command{
		Use:   "all",
		Short: "查询全部备份记录",
		Long: `从本地索引查询全部 MySQL 和 PostgreSQL 备份记录。

默认以表格显示最近 5 条记录。
使用 --last 调整显示数量。
使用 --json 输出机器可读格式。`,
		Example: `  dbbackupctl show all
  dbbackupctl show all --last 20
  dbbackupctl show all --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow("all", last, "", json)
		},
	}

	cmd.Flags().IntVar(&last, "last", 5, "显示记录数量")
	cmd.Flags().BoolVar(&json, "json", false, "以 JSON 格式输出")

	return cmd
}

func runShow(dbType string, last int, job string, jsonOutput bool) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create index store
	store := index.NewStore(cfg.Core.IndexFile)

	// Query records
	records, err := store.Query(index.QueryFilter{
		DBType: dbType,
		Job:    job,
		Limit:  last,
	})
	if err != nil {
		return fmt.Errorf("查询索引失败: %w", err)
	}

	// Output as JSON
	if jsonOutput {
		return printJSON(records)
	}

	// Output as table
	return printTable(records)
}

func printJSON(records []index.BackupRecord) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printTable(records []index.BackupRecord) error {
	if len(records) == 0 {
		fmt.Println("未找到备份记录。")
		return nil
	}

	headers := []string{"备份ID", "类型", "环境", "状态", "开始时间", "耗时", "大小", "路径"}
	rows := make([][]string, 0, len(records))
	for _, r := range records {
		rows = append(rows, []string{
			r.BackupID,
			r.DBType,
			r.Job,
			formatStatus(r.Status),
			r.StartedAt.Format("2006-01-02 15:04:05"),
			formatDuration(r.DurationSec),
			formatSize(r.SizeBytes),
			r.BackupDir,
		})
	}

	renderBoxTable(headers, rows)
	return nil
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%d分%d秒", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%d时%d分%d秒", seconds/3600, (seconds%3600)/60, seconds%60)
}

func formatStatus(status string) string {
	switch status {
	case "success":
		return "成功"
	case "failed":
		return "失败"
	case "running":
		return "运行中"
	default:
		return status
	}
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func renderBoxTable(headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = displayWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}

	printTableBorder("┌", "┬", "┐", widths)
	printTableRow(headers, widths)
	printTableBorder("├", "┼", "┤", widths)
	for _, row := range rows {
		printTableRow(row, widths)
	}
	printTableBorder("└", "┴", "┘", widths)
}

func printTableBorder(left, middle, right string, widths []int) {
	fmt.Print(left)
	for i, width := range widths {
		if i > 0 {
			fmt.Print(middle)
		}
		fmt.Print(strings.Repeat("─", width+2))
	}
	fmt.Println(right)
}

func printTableRow(row []string, widths []int) {
	fmt.Print("│")
	for i, cell := range row {
		fmt.Printf(" %s │", padRight(cell, widths[i]))
	}
	fmt.Println()
}

func padRight(s string, width int) string {
	padding := width - displayWidth(s)
	if padding <= 0 {
		return s
	}
	return s + strings.Repeat(" ", padding)
}

func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		switch {
		case r == '\t':
			width += 4
		case r < 0x20:
			continue
		case r <= 0x7e:
			width++
		default:
			width += 2
		}
	}
	return width
}
