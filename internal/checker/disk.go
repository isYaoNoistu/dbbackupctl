package checker

import (
	"context"
	"fmt"
	"os"

	"github.com/isYaoNoistu/dbbackupctl/internal/app"
	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
	"github.com/isYaoNoistu/dbbackupctl/internal/disk"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine/mysql"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine/postgresql"
)

// checkDiskSpace checks disk space
func (c *Checker) checkDiskSpace(report *CheckReport) {
	if !c.Config.Core.DiskGuardEnabled {
		c.addItem(report, "disk.guard", CheckWarn,
			"磁盘保护已关闭", "")
		return
	}

	// Parse minimum free size
	minFreeSize, err := configenv.ParseSize(c.Config.Core.DiskMinFreeSize)
	if err != nil {
		c.addItem(report, "disk.min_free_size", CheckFail,
			fmt.Sprintf("DBB_DISK_MIN_FREE_SIZE 配置非法: %s", c.Config.Core.DiskMinFreeSize),
			err.Error())
		return
	}

	// Check backup root disk space
	backupRoot := c.Config.Core.BackupRoot
	if backupRoot == "" {
		backupRoot = "/data/backup"
	}

	usage, err := disk.GetDiskUsage(backupRoot)
	if err != nil {
		c.addItem(report, "disk.space", CheckWarn,
			fmt.Sprintf("无法检查 %s 的磁盘空间", backupRoot),
			err.Error())
		return
	}

	// Check minimum free space
	if usage.Free < minFreeSize {
		c.addItem(report, "disk.space", CheckFail,
			fmt.Sprintf("磁盘空间不足：剩余 %s，要求 %s",
				disk.FormatBytes(usage.Free), disk.FormatBytes(minFreeSize)),
			fmt.Sprintf("总量: %s，已用: %s", disk.FormatBytes(usage.Total), disk.FormatBytes(usage.Used)))
		return
	}

	// Check free percentage
	freePercent := float64(usage.Free) / float64(usage.Total) * 100
	if freePercent < float64(c.Config.Core.DiskMinFreePercent) {
		c.addItem(report, "disk.space", CheckFail,
			fmt.Sprintf("磁盘空间比例不足：剩余 %.1f%%，要求 %d%%",
				freePercent, c.Config.Core.DiskMinFreePercent),
			fmt.Sprintf("剩余: %s，总量: %s", disk.FormatBytes(usage.Free), disk.FormatBytes(usage.Total)))
		return
	}

	c.addItem(report, "disk.space", CheckOK,
		fmt.Sprintf("磁盘空间正常：剩余 %s（%.1f%%）",
			disk.FormatBytes(usage.Free), freePercent),
		fmt.Sprintf("总量: %s，已用: %s", disk.FormatBytes(usage.Total), disk.FormatBytes(usage.Used)))
}

// checkMySQLConnection checks MySQL connection
func (c *Checker) checkMySQLConnection(ctx context.Context, report *CheckReport) {
	if !c.Config.MySQL.Enabled {
		return
	}

	matched := false
	for _, jobName := range c.Config.MySQL.Jobs {
		if c.JobName != "" && c.JobName != jobName {
			continue
		}
		matched = true

		job, ok := c.Config.MySQL.JobConfigs[jobName]
		if !ok {
			continue
		}

		// Try to resolve password
		_, err := resolvePassword(c.Config, job.PasswordEnv, job.PasswordFile)
		if err != nil {
			c.addItem(report, fmt.Sprintf("mysql.%s.password", jobName), CheckFail,
				fmt.Sprintf("无法解析环境 %s 的 MySQL 密码", jobName),
				err.Error())
			continue
		}

		c.addItem(report, fmt.Sprintf("mysql.%s.password", jobName), CheckOK,
			fmt.Sprintf("环境 %s 的 MySQL 密码已解析", jobName), "")

		engineJob, target, err := app.ConvertMySQLJob(c.Config, jobName)
		if err != nil {
			c.addItem(report, fmt.Sprintf("mysql.%s.config", jobName), CheckFail,
				fmt.Sprintf("无法解析 MySQL 环境 %s", jobName), err.Error())
			continue
		}
		eng := mysql.NewEngine()
		if err := eng.CheckConnection(ctx, engineJob); err != nil {
			c.addItem(report, fmt.Sprintf("mysql.%s.connection", jobName), CheckFail,
				fmt.Sprintf("MySQL 环境 %s 连接失败", jobName), err.Error())
			continue
		}
		c.addItem(report, fmt.Sprintf("mysql.%s.connection", jobName), CheckOK,
			fmt.Sprintf("MySQL 环境 %s 连接正常", jobName), "")
		if _, err := eng.EstimateSize(ctx, engineJob, target.Databases); err != nil {
			c.addItem(report, fmt.Sprintf("mysql.%s.estimate", jobName), CheckWarn,
				fmt.Sprintf("MySQL 环境 %s 估算大小失败", jobName), err.Error())
		} else {
			c.addItem(report, fmt.Sprintf("mysql.%s.estimate", jobName), CheckOK,
				fmt.Sprintf("MySQL 环境 %s 估算大小正常", jobName), "")
		}
	}
	if c.JobName != "" && !matched {
		c.addItem(report, fmt.Sprintf("mysql.%s.config", c.JobName), CheckFail,
			fmt.Sprintf("MySQL 配置中未找到环境 %s", c.JobName),
			"请检查 MYSQL_JOBS 和 MYSQL_<JOB>_* 配置")
	}
}

// checkPostgreSQLConnection checks PostgreSQL connection
func (c *Checker) checkPostgreSQLConnection(ctx context.Context, report *CheckReport) {
	if !c.Config.PostgreSQL.Enabled {
		return
	}

	matched := false
	for _, jobName := range c.Config.PostgreSQL.Jobs {
		if c.JobName != "" && c.JobName != jobName {
			continue
		}
		matched = true

		job, ok := c.Config.PostgreSQL.JobConfigs[jobName]
		if !ok {
			continue
		}

		// Try to resolve password
		_, err := resolvePassword(c.Config, job.PasswordEnv, job.PasswordFile)
		if err != nil {
			c.addItem(report, fmt.Sprintf("postgresql.%s.password", jobName), CheckFail,
				fmt.Sprintf("无法解析环境 %s 的 PostgreSQL 密码", jobName),
				err.Error())
			continue
		}

		c.addItem(report, fmt.Sprintf("postgresql.%s.password", jobName), CheckOK,
			fmt.Sprintf("环境 %s 的 PostgreSQL 密码已解析", jobName), "")

		engineJob, target, err := app.ConvertPostgreSQLJob(c.Config, jobName)
		if err != nil {
			c.addItem(report, fmt.Sprintf("postgresql.%s.config", jobName), CheckFail,
				fmt.Sprintf("无法解析 PostgreSQL 环境 %s", jobName), err.Error())
			continue
		}
		eng := postgresql.NewEngine()
		if err := eng.CheckConnection(ctx, engineJob); err != nil {
			c.addItem(report, fmt.Sprintf("postgresql.%s.connection", jobName), CheckFail,
				fmt.Sprintf("PostgreSQL 环境 %s 连接失败", jobName), err.Error())
			continue
		}
		c.addItem(report, fmt.Sprintf("postgresql.%s.connection", jobName), CheckOK,
			fmt.Sprintf("PostgreSQL 环境 %s 连接正常", jobName), "")
		if _, err := eng.EstimateSize(ctx, engineJob, target.Databases); err != nil {
			c.addItem(report, fmt.Sprintf("postgresql.%s.estimate", jobName), CheckWarn,
				fmt.Sprintf("PostgreSQL 环境 %s 估算大小失败", jobName), err.Error())
		} else {
			c.addItem(report, fmt.Sprintf("postgresql.%s.estimate", jobName), CheckOK,
				fmt.Sprintf("PostgreSQL 环境 %s 估算大小正常", jobName), "")
		}
	}
	if c.JobName != "" && !matched {
		c.addItem(report, fmt.Sprintf("postgresql.%s.config", c.JobName), CheckFail,
			fmt.Sprintf("PostgreSQL 配置中未找到环境 %s", c.JobName),
			"请检查 POSTGRES_JOBS 和 POSTGRES_<JOB>_* 配置")
	}
}

// checkLocks checks for stale locks
func (c *Checker) checkLocks(report *CheckReport) {
	lockDir := c.Config.Core.LockDir
	if lockDir == "" {
		lockDir = "/var/lock/dbbackupctl"
	}

	c.addItem(report, "lock.check", CheckOK,
		fmt.Sprintf("锁目录：%s", lockDir), "")
}

// resolvePassword resolves password from environment or secret config
func resolvePassword(cfg *configenv.Config, envName, filePath string) (string, error) {
	// Try system environment variable
	if v := os.Getenv(envName); v != "" {
		return v, nil
	}

	// Try secret.env
	if v, ok := cfg.Secret.GetPassword(envName); ok && v != "" {
		return v, nil
	}

	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("读取密码文件 %s 失败: %w", filePath, err)
		}
		if v := string(b); v != "" {
			return v, nil
		}
	}

	return "", fmt.Errorf("未找到密码: %s", envName)
}
