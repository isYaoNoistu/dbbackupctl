package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/isYaoNoistu/dbbackupctl/internal/index"
	"github.com/isYaoNoistu/dbbackupctl/internal/manifest"
	"github.com/spf13/cobra"
)

func newIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "管理本地备份索引",
		Long: `管理本地备份索引 backup_records.jsonl。

子命令：
  verify  校验索引完整性
  rebuild 从备份目录重建索引

默认索引位置：
  /data/dbbackupctl/index/backup_records.jsonl`,
		Example: `  dbbackupctl index verify
  dbbackupctl index rebuild
  dbbackupctl index rebuild --backup-root /data/backup`,
	}

	cmd.AddCommand(
		newIndexVerifyCmd(),
		newIndexRebuildCmd(),
	)

	return cmd
}

func newIndexVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "校验索引完整性",
		Long: `校验备份索引完整性。

检查内容：
  - 索引文件可读
  - 每条记录包含必要字段
  - 引用的 manifest.json 存在
  - 引用的备份目录存在`,
		Example: `  dbbackupctl index verify`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIndexVerify()
		},
	}

	return cmd
}

func newIndexRebuildCmd() *cobra.Command {
	var backupRoot string

	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "从备份目录重建索引",
		Long: `扫描备份目录并重建本地备份索引。

该命令会：
  - 扫描 backup_root 下的所有备份目录
  - 读取每个备份的 manifest.json
  - 重建 backup_records.jsonl

适用场景：
  - 索引文件损坏或丢失
  - 手工移动过备份目录
  - 迁移过备份根目录`,
		Example: `  dbbackupctl index rebuild
  dbbackupctl index rebuild --backup-root /data/backup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIndexRebuild(backupRoot)
		},
	}

	cmd.Flags().StringVar(&backupRoot, "backup-root", "", "备份根目录，默认读取配置")

	return cmd
}

func runIndexVerify() error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create index store
	store := index.NewStore(cfg.Core.IndexFile)

	// Query all records
	records, err := store.Query(index.QueryFilter{Limit: 10000})
	if err != nil {
		return fmt.Errorf("读取索引失败: %w", err)
	}

	fmt.Printf("索引中共有 %d 条记录\n", len(records))

	// Verify each record
	issues := 0
	for _, r := range records {
		// Check required fields
		if r.BackupID == "" {
			fmt.Printf("  问题：记录缺少 backup_id\n")
			issues++
			continue
		}

		// Check manifest exists
		manifestPath := filepath.Join(r.BackupDir, "manifest.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			fmt.Printf("  问题：%s - 未找到 manifest：%s\n", r.BackupID, manifestPath)
			issues++
			continue
		}

		// Check backup dir exists
		if _, err := os.Stat(r.BackupDir); os.IsNotExist(err) {
			fmt.Printf("  问题：%s - 未找到备份目录：%s\n", r.BackupID, r.BackupDir)
			issues++
			continue
		}

		// Read and verify manifest
		mw := manifest.NewWriter()
		m, err := mw.Read(r.BackupDir)
		if err != nil {
			fmt.Printf("  问题：%s - 无法读取 manifest：%v\n", r.BackupID, err)
			issues++
			continue
		}

		// Verify manifest matches index
		if m.BackupID != r.BackupID {
			fmt.Printf("  问题：%s - backup_id 与 manifest 不一致\n", r.BackupID)
			issues++
			continue
		}

		fmt.Printf("  通过：%s\n", r.BackupID)
	}

	if issues > 0 {
		fmt.Printf("\n发现 %d 个问题。可执行 'dbbackupctl index rebuild' 修复。\n", issues)
	} else {
		fmt.Printf("\n所有记录校验通过。\n")
	}

	return nil
}

func runIndexRebuild(backupRoot string) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Use configured backup root if not specified
	if backupRoot == "" {
		backupRoot = cfg.Core.BackupRoot
		if backupRoot == "" {
			backupRoot = "/data/backup"
		}
	}

	fmt.Printf("正在扫描备份目录：%s\n", backupRoot)

	store := index.NewStore(cfg.Core.IndexFile)
	if err := store.Rebuild(backupRoot); err != nil {
		return fmt.Errorf("重建索引失败: %w", err)
	}

	records, err := store.Query(index.QueryFilter{Limit: 100000})
	if err != nil {
		return fmt.Errorf("读取重建后的索引失败: %w", err)
	}

	fmt.Printf("\n索引重建完成：%s\n", cfg.Core.IndexFile)
	fmt.Printf("记录总数：%d\n", len(records))

	return nil
}
