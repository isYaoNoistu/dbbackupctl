package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/isYaoNoistu/dbbackupctl/internal/checker"
	"github.com/isYaoNoistu/dbbackupctl/internal/exiterr"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	var (
		mysql      bool
		postgresql bool
		job        string
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "检查配置、依赖、权限和磁盘空间",
		Long: `检查 dbbackupctl 的运行环境和配置。

检查内容：
  - 配置文件存在且合法
  - 环境变量和密码配置可解析
  - secret.env 权限是否安全
  - 备份目录是否存在且可写
  - mysqldump、pg_dump、zstd 等依赖是否可用
  - 数据库连接是否可用
  - 磁盘空间是否满足阈值
  - 锁目录是否正常`,
		Example: `  dbbackupctl check
  dbbackupctl check --mysql --job dev
  dbbackupctl check --postgresql --job dev
  dbbackupctl check --job prod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(mysql, postgresql, job)
		},
	}

	cmd.Flags().BoolVar(&mysql, "mysql", false, "只检查 MySQL 配置")
	cmd.Flags().BoolVar(&postgresql, "postgresql", false, "只检查 PostgreSQL 配置")
	cmd.Flags().StringVar(&job, "job", "", "只检查指定环境，例如 dev、prod")

	return cmd
}

func runCheck(mysql, postgresql bool, job string) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		// If config loading fails, try to create a basic check report
		fmt.Fprintf(os.Stderr, "警告：无法加载配置：%v\n", err)
		fmt.Println("检查失败：配置错误")
		return exiterr.New(exiterr.ExitConfig, err)
	}

	// Create checker
	ch := checker.NewChecker(cfg, GetConfigDir())
	ch.CheckMySQL = mysql
	ch.CheckPg = postgresql
	ch.JobName = job

	// Run checks
	report := ch.Run(context.Background())

	// Output as JSON if requested
	if IsJSONOutput() {
		return printCheckJSON(report)
	}

	// Output as table
	printCheckTable(report)

	// Return error if any check failed
	if report.HasFailure() {
		return exiterr.New(exiterr.ExitGeneral, fmt.Errorf("检查失败"))
	}

	return nil
}

func printCheckJSON(report *checker.CheckReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printCheckTable(report *checker.CheckReport) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintf(w, "状态\t检查项\t说明\n")
	fmt.Fprintf(w, "----\t------\t----\n")

	// Print items
	for _, item := range report.Items {
		status := string(item.Status)
		switch item.Status {
		case checker.CheckOK:
			status = "通过"
		case checker.CheckWarn:
			status = "警告"
		case checker.CheckFail:
			status = "失败"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", status, item.Name, item.Message)
	}

	w.Flush()

	// Print summary
	fmt.Printf("\n%s\n", report.Summary())
}
