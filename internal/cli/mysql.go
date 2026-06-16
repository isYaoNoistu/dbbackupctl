package cli

import (
	"context"
	"fmt"

	"github.com/isYaoNoistu/dbbackupctl/internal/app"
	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
	"github.com/spf13/cobra"
)

func newMySQLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mysql",
		Short: "MySQL 备份和恢复命令",
		Long: `MySQL 备份和恢复操作。

子命令：
  backup    创建 MySQL 备份
  restore   恢复 MySQL 备份

配置：
  MySQL 多环境配置在 /etc/dbbackupctl/mysql.env
  --job 表示环境名，例如 dev、uat、prod
  密码从当前进程环境变量、secret.env 或 password_file 读取`,
		Example: `  dbbackupctl mysql backup --job dev
  dbbackupctl mysql backup --all
  dbbackupctl mysql backup --job prod --dry-run
  dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore
  dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore --execute`,
	}

	cmd.AddCommand(
		newMySQLBackupCmd(),
		newMySQLRestoreCmd(),
	)

	return cmd
}

func newMySQLBackupCmd() *cobra.Command {
	var (
		job        string
		all        bool
		dryRun     bool
		noCompress bool
		noPrune    bool
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "创建 MySQL 备份",
		Long: `使用 mysqldump 创建 MySQL 逻辑备份。

默认使用 zstd 压缩，并写入配置中指定的备份目录。
每次备份都会生成 manifest.json 和本地索引记录。

密码读取顺序：
  1. MYSQL_<JOB>_PASSWORD_ENV 指定的当前进程环境变量
  2. /etc/dbbackupctl/secret.env 中的值
  3. MYSQL_<JOB>_PASSWORD_FILE 指定的文件

示例：
  dbbackupctl mysql backup --job dev
  dbbackupctl mysql backup --all
  dbbackupctl mysql backup --job prod --dry-run
  dbbackupctl mysql backup --job dev --no-compress`,
		Example: `  dbbackupctl mysql backup --job dev
  dbbackupctl mysql backup --all
  dbbackupctl mysql backup --job prod --dry-run
  dbbackupctl mysql backup --job dev --no-compress
  dbbackupctl mysql backup --job prod --no-prune
  dbbackupctl mysql backup --job dev --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMySQLBackup(job, all, dryRun, noCompress, noPrune, force)
		},
	}

	cmd.Flags().StringVar(&job, "job", "", "要备份的环境名，例如 dev、prod；除非使用 --all，否则必填")
	cmd.Flags().BoolVar(&all, "all", false, "备份所有已启用环境")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只显示备份计划，不执行")
	cmd.Flags().BoolVar(&noCompress, "no-compress", false, "禁用压缩，仅用于调试")
	cmd.Flags().BoolVar(&noPrune, "no-prune", false, "备份后不执行保留策略清理")
	cmd.Flags().BoolVar(&force, "force", false, "执行前清理陈旧锁")

	return cmd
}

func newMySQLRestoreCmd() *cobra.Command {
	var (
		id             string
		sourceDB       string
		targetDB       string
		execute        bool
		allowOverwrite bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "恢复 MySQL 备份",
		Long: `将 MySQL 备份恢复到目标数据库。

默认只输出恢复计划，不执行真实恢复。
需要真正执行时必须显式增加 --execute。

安全机制：
  - 恢复前校验 checksum
  - 恢复到源库必须显式增加 --allow-overwrite
  - 恢复记录会写入审计文件

示例：
  dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore
  dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore --execute
  dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app --allow-overwrite --execute`,
		Example: `  dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore
  dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore --execute
  dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app --allow-overwrite --execute`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMySQLRestore(id, sourceDB, targetDB, execute, allowOverwrite)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "要恢复的备份 ID，必填")
	cmd.Flags().StringVar(&sourceDB, "source-db", "", "源数据库名；备份包含多个数据库时必填")
	cmd.Flags().StringVar(&sourceDB, "database", "", "源数据库名，等同于 --source-db")
	cmd.Flags().StringVar(&targetDB, "target-db", "", "目标数据库名，必填")
	cmd.Flags().BoolVar(&execute, "execute", false, "执行恢复；默认只输出计划")
	cmd.Flags().BoolVar(&allowOverwrite, "allow-overwrite", false, "允许覆盖源数据库")

	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("target-db")

	return cmd
}

func runMySQLBackup(job string, all, dryRun, noCompress, noPrune, force bool) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create backup runner
	runner := app.NewBackupRunner(cfg)

	// Build options
	opt := app.BackupOptions{
		DryRun:     dryRun,
		NoCompress: noCompress,
		NoPrune:    noPrune,
		Force:      force,
	}

	// Backup all jobs
	if all {
		for _, j := range cfg.MySQL.Jobs {
			if err := runner.BackupMySQL(context.Background(), j, opt); err != nil {
				return err
			}
		}
		return nil
	}

	// Backup single job
	if job == "" {
		return fmt.Errorf("必须指定 --job，除非使用 --all")
	}

	return runner.BackupMySQL(context.Background(), job, opt)
}

func runMySQLRestore(id, sourceDB, targetDB string, execute, allowOverwrite bool) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create restore runner
	runner := app.NewRestoreRunner(cfg)

	// Build options
	opt := app.RestoreOptions{
		SourceDB:       sourceDB,
		TargetDB:       targetDB,
		Execute:        execute,
		AllowOverwrite: allowOverwrite,
	}

	return runner.RestoreMySQL(context.Background(), id, opt)
}

func loadConfig() (*configenv.Config, error) {
	// Use global config directory
	loader := configenv.NewLoader(GetConfigDir())
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	// Validate configuration
	validator := configenv.NewValidator()
	if err := validator.Validate(cfg); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}

	return cfg, nil
}
