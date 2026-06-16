package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
	"github.com/isYaoNoistu/dbbackupctl/internal/index"
	"github.com/isYaoNoistu/dbbackupctl/internal/retention"
	"github.com/spf13/cobra"
)

func newPruneCmd() *cobra.Command {
	var (
		mysql      bool
		postgresql bool
		job        string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "按保留策略清理旧备份",
		Long: `按保留策略删除旧备份。

保留策略在 core.env 中配置：
  - DBB_RETENTION_KEEP_LAST：保留最近 N 个备份
  - DBB_RETENTION_KEEP_DAYS：保留最近 N 天备份
  - DBB_RETENTION_MAX_TOTAL_SIZE：限制总大小

该命令也会按 DBB_RETENTION_KEEP_FAILED_LAST 清理失败备份。`,
		Example: `  dbbackupctl prune
  dbbackupctl prune --mysql
  dbbackupctl prune --postgresql
  dbbackupctl prune --job dev
  dbbackupctl prune --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrune(mysql, postgresql, job, dryRun)
		},
	}

	cmd.Flags().BoolVar(&mysql, "mysql", false, "只清理 MySQL 备份")
	cmd.Flags().BoolVar(&postgresql, "postgresql", false, "只清理 PostgreSQL 备份")
	cmd.Flags().StringVar(&job, "job", "", "只清理指定环境，例如 dev、prod")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只显示清理计划，不删除")

	return cmd
}

func runPrune(mysql, postgresql bool, job string, dryRun bool) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Parse max total size
	var maxSize int64
	if cfg.Core.RetentionMaxTotalSize != "" {
		maxSize, err = configenv.ParseSize(cfg.Core.RetentionMaxTotalSize)
		if err != nil {
			return fmt.Errorf("解析最大保留大小失败: %w", err)
		}
	}

	// Create index store
	store := index.NewStore(cfg.Core.IndexFile)

	// Query all records
	records, err := store.Query(index.QueryFilter{
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("查询索引失败: %w", err)
	}

	// Filter by type
	var filtered []index.BackupRecord
	for _, r := range records {
		if mysql && r.DBType != "mysql" {
			continue
		}
		if postgresql && r.DBType != "postgresql" {
			continue
		}
		if job != "" && r.Job != job {
			continue
		}
		filtered = append(filtered, r)
	}

	// Group by job
	jobRecords := make(map[string][]index.BackupRecord)
	for _, r := range filtered {
		key := r.DBType + "/" + r.Job
		jobRecords[key] = append(jobRecords[key], r)
	}

	// Apply retention policies
	keeper := retention.NewKeeper(
		cfg.Core.RetentionKeepLast,
		cfg.Core.RetentionKeepDays,
		maxSize,
		cfg.Core.RetentionKeepFailedLast,
	)

	var toDelete []index.BackupRecord
	for _, records := range jobRecords {
		// Sort by start time (newest first)
		sort.Slice(records, func(i, j int) bool {
			return records[i].StartedAt.After(records[j].StartedAt)
		})

		// Convert to retention records
		retRecords := make([]retention.BackupRecord, len(records))
		for i, r := range records {
			retRecords[i] = retention.BackupRecord{
				BackupID:  r.BackupID,
				DBType:    r.DBType,
				Job:       r.Job,
				Status:    r.Status,
				StartedAt: r.StartedAt,
				SizeBytes: r.SizeBytes,
				BackupDir: r.BackupDir,
			}
		}

		// Apply retention policy
		toPrune := keeper.Prune(retRecords)

		// Convert back to index records
		for _, p := range toPrune {
			for _, r := range records {
				if r.BackupID == p.BackupID {
					toDelete = append(toDelete, r)
					break
				}
			}
		}
	}

	// Dry run mode
	if dryRun {
		fmt.Printf("预演模式：将删除 %d 个备份：\n", len(toDelete))
		for _, r := range toDelete {
			fmt.Printf("  %s (%s/%s) - %s\n", r.BackupID, r.DBType, r.Job, r.BackupDir)
		}
		return nil
	}

	// Delete backups
	deleted := 0
	for _, r := range toDelete {
		if err := deleteBackupDir(cfg.Core.BackupRoot, r); err != nil {
			fmt.Fprintf(os.Stderr, "警告：删除 %s 失败：%v\n", r.BackupDir, err)
			continue
		}
		deleted++
	}

	fmt.Printf("已清理 %d 个备份\n", deleted)
	if deleted > 0 {
		if err := store.Rebuild(cfg.Core.BackupRoot); err != nil {
			return fmt.Errorf("清理后重建索引失败: %w", err)
		}
		fmt.Println("清理后已重建索引")
	}
	return nil
}

// deleteBackupDir deletes a backup directory after safety checks.
func deleteBackupDir(backupRoot string, record index.BackupRecord) error {
	dir := record.BackupDir
	if dir == "" {
		return fmt.Errorf("备份目录为空: %s", record.BackupID)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	rootAbs, err := filepath.Abs(backupRoot)
	if err != nil {
		return fmt.Errorf("解析备份根目录失败: %w", err)
	}
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("解析备份目录失败: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, dirAbs)
	if err != nil {
		return fmt.Errorf("检查备份目录失败: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("拒绝删除备份根目录之外的路径: %s", dir)
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("删除前未找到 manifest.json: %w", err)
	}

	return os.RemoveAll(dir)
}
