package checker

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// checkDependencies checks required dependencies
func (c *Checker) checkDependencies(report *CheckReport) {
	// Check compression tools based on config
	compressType := c.Config.Core.CompressType
	if compressType == "" {
		compressType = "zstd"
	}

	switch compressType {
	case "zstd":
		c.checkBinary(report, "dependency.zstd", "zstd")
	case "gzip":
		c.checkBinary(report, "dependency.gzip", "gzip")
	}

	// Check MySQL dependencies if enabled
	if c.Config.MySQL.Enabled {
		c.checkBinary(report, "dependency.mysqldump", "mysqldump")
		c.checkBinary(report, "dependency.mysql", "mysql")
	}

	// Check PostgreSQL dependencies if enabled
	if c.Config.PostgreSQL.Enabled {
		c.checkBinary(report, "dependency.pg_dump", "pg_dump")
		c.checkBinary(report, "dependency.pg_restore", "pg_restore")
		c.checkBinary(report, "dependency.pg_dumpall", "pg_dumpall")
		c.checkBinary(report, "dependency.psql", "psql")
	}
}

// checkBinary checks if a binary exists in PATH
func (c *Checker) checkBinary(report *CheckReport, name, binary string) {
	path, err := exec.LookPath(binary)
	if err != nil {
		c.addItem(report, name, CheckFail,
			fmt.Sprintf("%s not found in PATH", binary),
			"Please install the required tool")
		return
	}
	c.addItem(report, name, CheckOK,
		fmt.Sprintf("%s found at %s", binary, path), "")
}

// checkDirectories checks required directories
func (c *Checker) checkDirectories(report *CheckReport) {
	dirs := []struct {
		name    string
		path    string
		require bool
	}{
		{"data_dir", c.Config.Core.DataDir, true},
		{"backup_root", c.Config.Core.BackupRoot, true},
		{"tmp_dir", c.Config.Core.TmpDir, false},
		{"log_dir", c.Config.Core.LogDir, false},
		{"lock_dir", c.Config.Core.LockDir, true},
		{"index_dir", filepath.Dir(c.Config.Core.IndexFile), true},
	}

	for _, d := range dirs {
		if d.path == "" {
			continue
		}
		if _, err := exec.Command("test", "-d", d.path).Output(); err != nil {
			if d.require {
				c.addItem(report, fmt.Sprintf("directory.%s", d.name), CheckFail,
					fmt.Sprintf("Directory does not exist: %s", d.path),
					"Run: mkdir -p "+d.path)
			} else {
				c.addItem(report, fmt.Sprintf("directory.%s", d.name), CheckWarn,
					fmt.Sprintf("Directory does not exist: %s", d.path), "")
			}
		} else {
			c.addItem(report, fmt.Sprintf("directory.%s", d.name), CheckOK,
				fmt.Sprintf("Directory exists: %s", d.path), "")
		}
	}
}