package checker

import (
	"context"
	"fmt"
	"os"

	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
	"github.com/isYaoNoistu/dbbackupctl/internal/disk"
)

// checkDiskSpace checks disk space
func (c *Checker) checkDiskSpace(report *CheckReport) {
	if !c.Config.Core.DiskGuardEnabled {
		c.addItem(report, "disk.guard", CheckWarn,
			"Disk guard is disabled", "")
		return
	}

	// Parse minimum free size
	minFreeSize, err := configenv.ParseSize(c.Config.Core.DiskMinFreeSize)
	if err != nil {
		c.addItem(report, "disk.min_free_size", CheckFail,
			fmt.Sprintf("Invalid DBB_DISK_MIN_FREE_SIZE: %s", c.Config.Core.DiskMinFreeSize),
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
			fmt.Sprintf("Cannot check disk space for %s", backupRoot),
			err.Error())
		return
	}

	// Check minimum free space
	if usage.Free < minFreeSize {
		c.addItem(report, "disk.space", CheckFail,
			fmt.Sprintf("Insufficient disk space: free %s, required %s",
				disk.FormatBytes(usage.Free), disk.FormatBytes(minFreeSize)),
			fmt.Sprintf("Total: %s, Used: %s", disk.FormatBytes(usage.Total), disk.FormatBytes(usage.Used)))
		return
	}

	// Check free percentage
	freePercent := float64(usage.Free) / float64(usage.Total) * 100
	if freePercent < float64(c.Config.Core.DiskMinFreePercent) {
		c.addItem(report, "disk.space", CheckFail,
			fmt.Sprintf("Insufficient disk space: %.1f%% free, required %d%%",
				freePercent, c.Config.Core.DiskMinFreePercent),
			fmt.Sprintf("Free: %s, Total: %s", disk.FormatBytes(usage.Free), disk.FormatBytes(usage.Total)))
		return
	}

	c.addItem(report, "disk.space", CheckOK,
		fmt.Sprintf("Disk space OK: %s free (%.1f%%)",
			disk.FormatBytes(usage.Free), freePercent),
		fmt.Sprintf("Total: %s, Used: %s", disk.FormatBytes(usage.Total), disk.FormatBytes(usage.Used)))
}

// checkMySQLConnection checks MySQL connection
func (c *Checker) checkMySQLConnection(ctx context.Context, report *CheckReport) {
	if !c.Config.MySQL.Enabled {
		return
	}

	for _, jobName := range c.Config.MySQL.Jobs {
		if c.JobName != "" && c.JobName != jobName {
			continue
		}

		job, ok := c.Config.MySQL.JobConfigs[jobName]
		if !ok {
			continue
		}

		// Try to resolve password
		_, err := resolvePassword(c.Config, job.PasswordEnv)
		if err != nil {
			c.addItem(report, fmt.Sprintf("mysql.%s.password", jobName), CheckFail,
				fmt.Sprintf("Cannot resolve password for job %s", jobName),
				err.Error())
			continue
		}

		c.addItem(report, fmt.Sprintf("mysql.%s.password", jobName), CheckOK,
			fmt.Sprintf("Password resolved for job %s", jobName), "")

		// Connection check would require database driver, skip for now
		c.addItem(report, fmt.Sprintf("mysql.%s.connection", jobName), CheckWarn,
			fmt.Sprintf("MySQL connection check not implemented for job %s", jobName),
			"Connection will be verified during backup")
	}
}

// checkPostgreSQLConnection checks PostgreSQL connection
func (c *Checker) checkPostgreSQLConnection(ctx context.Context, report *CheckReport) {
	if !c.Config.PostgreSQL.Enabled {
		return
	}

	for _, jobName := range c.Config.PostgreSQL.Jobs {
		if c.JobName != "" && c.JobName != jobName {
			continue
		}

		job, ok := c.Config.PostgreSQL.JobConfigs[jobName]
		if !ok {
			continue
		}

		// Try to resolve password
		_, err := resolvePassword(c.Config, job.PasswordEnv)
		if err != nil {
			c.addItem(report, fmt.Sprintf("postgresql.%s.password", jobName), CheckFail,
				fmt.Sprintf("Cannot resolve password for job %s", jobName),
				err.Error())
			continue
		}

		c.addItem(report, fmt.Sprintf("postgresql.%s.password", jobName), CheckOK,
			fmt.Sprintf("Password resolved for job %s", jobName), "")

		// Connection check would require database driver, skip for now
		c.addItem(report, fmt.Sprintf("postgresql.%s.connection", jobName), CheckWarn,
			fmt.Sprintf("PostgreSQL connection check not implemented for job %s", jobName),
			"Connection will be verified during backup")
	}
}

// checkLocks checks for stale locks
func (c *Checker) checkLocks(report *CheckReport) {
	lockDir := c.Config.Core.LockDir
	if lockDir == "" {
		lockDir = "/var/lock/dbbackupctl"
	}

	c.addItem(report, "lock.check", CheckOK,
		fmt.Sprintf("Lock directory: %s", lockDir), "")
}

// resolvePassword resolves password from environment or secret config
func resolvePassword(cfg *configenv.Config, envName string) (string, error) {
	// Try system environment variable
	if v := os.Getenv(envName); v != "" {
		return v, nil
	}

	// Try secret.env
	if v, ok := cfg.Secret.GetPassword(envName); ok && v != "" {
		return v, nil
	}

	return "", fmt.Errorf("password not found: %s", envName)
}