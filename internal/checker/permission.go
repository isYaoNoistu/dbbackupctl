package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// checkPermissions checks file permissions
func (c *Checker) checkPermissions(report *CheckReport) {
	// Skip permission checks on Windows
	if runtime.GOOS == "windows" {
		c.addItem(report, "permission.secret.env", CheckOK,
			"Windows 环境跳过权限检查", "")
		return
	}

	// Check secret.env permissions
	secretEnv := filepath.Join(c.ConfigDir, "secret.env")
	if info, err := os.Stat(secretEnv); err == nil {
		mode := info.Mode().Perm()
		// Should be 0600 or at most 0640
		if mode > 0640 {
			c.addItem(report, "permission.secret.env", CheckFail,
				fmt.Sprintf("secret.env 权限不安全: %04o", mode),
				"执行: chmod 600 /etc/dbbackupctl/secret.env")
		} else {
			c.addItem(report, "permission.secret.env", CheckOK,
				fmt.Sprintf("secret.env 权限: %04o", mode), "")
		}
	}

	// Check directory permissions
	dirs := []struct {
		name string
		path string
	}{
		{"config_dir", c.Config.Core.ConfigDir},
		{"data_dir", c.Config.Core.DataDir},
		{"backup_root", c.Config.Core.BackupRoot},
		{"log_dir", c.Config.Core.LogDir},
		{"lock_dir", c.Config.Core.LockDir},
	}

	for _, d := range dirs {
		if d.path == "" {
			continue
		}
		if _, err := os.Stat(d.path); err == nil {
			c.addItem(report, fmt.Sprintf("permission.%s", d.name), CheckOK,
				fmt.Sprintf("%s 存在", d.name), "")
		}
	}
}
