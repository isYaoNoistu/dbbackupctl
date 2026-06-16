package cli

import (
	"context"
	"fmt"

	"github.com/isYaoNoistu/dbbackupctl/internal/app"
	"github.com/spf13/cobra"
)

func newPostgreSQLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "postgresql",
		Short: "PostgreSQL 备份和恢复命令",
		Long: `PostgreSQL 备份和恢复操作。

子命令：
  backup    创建 PostgreSQL 备份
  restore   恢复 PostgreSQL 备份

配置：
  PostgreSQL 多环境配置在 /etc/dbbackupctl/postgresql.env
  --job 表示环境名，例如 dev、uat、prod
  密码从当前进程环境变量、secret.env 或 password_file 读取`,
		Example: `  dbbackupctl postgresql backup --job dev
  dbbackupctl postgresql backup --all
  dbbackupctl postgresql backup --job prod --dry-run
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore --execute`,
	}

	cmd.AddCommand(
		newPostgreSQLBackupCmd(),
		newPostgreSQLRestoreCmd(),
	)

	return cmd
}

func newPostgreSQLBackupCmd() *cobra.Command {
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
		Short: "创建 PostgreSQL 备份",
		Long: `使用 pg_dump 创建 PostgreSQL 逻辑备份。

默认使用 zstd 压缩，并写入配置中指定的备份目录。
每次备份都会生成 manifest.json 和本地索引记录。

密码读取顺序：
  1. POSTGRES_<JOB>_PASSWORD_ENV 指定的当前进程环境变量
  2. /etc/dbbackupctl/secret.env 中的值
  3. POSTGRES_<JOB>_PASSWORD_FILE 指定的文件

示例：
  dbbackupctl postgresql backup --job dev
  dbbackupctl postgresql backup --all
  dbbackupctl postgresql backup --job prod --dry-run
  dbbackupctl postgresql backup --job dev --no-compress`,
		Example: `  dbbackupctl postgresql backup --job dev
  dbbackupctl postgresql backup --all
  dbbackupctl postgresql backup --job prod --dry-run
  dbbackupctl postgresql backup --job dev --no-compress
  dbbackupctl postgresql backup --job prod --no-prune
  dbbackupctl postgresql backup --job dev --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPostgreSQLBackup(job, all, dryRun, noCompress, noPrune, force)
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

func newPostgreSQLRestoreCmd() *cobra.Command {
	var (
		id             string
		sourceDB       string
		targetDB       string
		execute        bool
		allowOverwrite bool
		includeGlobals bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "恢复 PostgreSQL 备份",
		Long: `将 PostgreSQL 备份恢复到目标数据库。

默认只输出恢复计划，不执行真实恢复。
需要真正执行时必须显式增加 --execute。

安全机制：
  - 恢复前校验 checksum
  - 恢复到源库必须显式增加 --allow-overwrite
  - PostgreSQL 全局对象默认不恢复，必须显式增加 --include-globals
  - 恢复记录会写入审计文件

示例：
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore --execute
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app --allow-overwrite --execute`,
		Example: `  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore --execute
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app --allow-overwrite --execute`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPostgreSQLRestore(id, sourceDB, targetDB, execute, allowOverwrite, includeGlobals)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "要恢复的备份 ID，必填")
	cmd.Flags().StringVar(&sourceDB, "source-db", "", "源数据库名；备份包含多个数据库时必填")
	cmd.Flags().StringVar(&sourceDB, "database", "", "源数据库名，等同于 --source-db")
	cmd.Flags().StringVar(&targetDB, "target-db", "", "目标数据库名，必填")
	cmd.Flags().BoolVar(&execute, "execute", false, "执行恢复；默认只输出计划")
	cmd.Flags().BoolVar(&allowOverwrite, "allow-overwrite", false, "允许覆盖源数据库")
	cmd.Flags().BoolVar(&includeGlobals, "include-globals", false, "恢复 PostgreSQL 全局对象到 postgres 数据库")

	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("target-db")

	return cmd
}

func runPostgreSQLBackup(job string, all, dryRun, noCompress, noPrune, force bool) error {
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
		for _, j := range cfg.PostgreSQL.Jobs {
			if err := runner.BackupPostgreSQL(context.Background(), j, opt); err != nil {
				return err
			}
		}
		return nil
	}

	// Backup single job
	if job == "" {
		return fmt.Errorf("必须指定 --job，除非使用 --all")
	}

	return runner.BackupPostgreSQL(context.Background(), job, opt)
}

func runPostgreSQLRestore(id, sourceDB, targetDB string, execute, allowOverwrite, includeGlobals bool) error {
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
		IncludeGlobals: includeGlobals,
	}

	return runner.RestorePostgreSQL(context.Background(), id, opt)
}
