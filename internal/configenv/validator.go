package configenv

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Validator handles configuration validation
type Validator struct{}

// NewValidator creates a new configuration validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates the entire configuration
func (v *Validator) Validate(cfg *Config) error {
	if err := v.validateCore(&cfg.Core); err != nil {
		return fmt.Errorf("core config: %w", err)
	}

	if cfg.MySQL.Enabled {
		if err := v.validateMySQL(&cfg.MySQL); err != nil {
			return fmt.Errorf("mysql config: %w", err)
		}
	}

	if cfg.PostgreSQL.Enabled {
		if err := v.validatePostgreSQL(&cfg.PostgreSQL); err != nil {
			return fmt.Errorf("postgresql config: %w", err)
		}
	}

	return nil
}

// validateCore validates core configuration
func (v *Validator) validateCore(cfg *CoreConfig) error {
	// Validate all directories are absolute paths
	dirs := map[string]string{
		"DBB_CONFIG_DIR":  cfg.ConfigDir,
		"DBB_DATA_DIR":    cfg.DataDir,
		"DBB_BACKUP_ROOT": cfg.BackupRoot,
		"DBB_TMP_DIR":     cfg.TmpDir,
		"DBB_LOG_DIR":     cfg.LogDir,
		"DBB_LOCK_DIR":    cfg.LockDir,
	}

	for name, dir := range dirs {
		if dir != "" && !filepath.IsAbs(dir) {
			return fmt.Errorf("%s must be an absolute path: %s", name, dir)
		}
	}

	// Validate backup root is not a dangerous path
	if err := v.validateBackupDir(cfg.BackupRoot); err != nil {
		return fmt.Errorf("DBB_BACKUP_ROOT: %w", err)
	}

	// Validate compress type
	validCompressTypes := map[string]bool{"zstd": true, "gzip": true, "none": true}
	if cfg.CompressType != "" && !validCompressTypes[cfg.CompressType] {
		return fmt.Errorf("DBB_COMPRESS_TYPE invalid: %s (must be zstd, gzip, or none)", cfg.CompressType)
	}

	// Validate checksum type
	if cfg.ChecksumType != "" && cfg.ChecksumType != "sha256" {
		return fmt.Errorf("DBB_CHECKSUM_TYPE invalid: %s (must be sha256)", cfg.ChecksumType)
	}

	// Validate compress level
	if cfg.CompressType == "zstd" && cfg.CompressLevel != 0 && (cfg.CompressLevel < 1 || cfg.CompressLevel > 22) {
		return fmt.Errorf("DBB_COMPRESS_LEVEL invalid for zstd: %d (must be 1-22)", cfg.CompressLevel)
	}
	if cfg.CompressType == "gzip" && cfg.CompressLevel != 0 && (cfg.CompressLevel < 1 || cfg.CompressLevel > 9) {
		return fmt.Errorf("DBB_COMPRESS_LEVEL invalid for gzip: %d (must be 1-9)", cfg.CompressLevel)
	}

	// Validate retention settings
	if cfg.RetentionKeepLast < 0 {
		return fmt.Errorf("DBB_RETENTION_KEEP_LAST must be >= 0")
	}
	if cfg.RetentionKeepDays < 0 {
		return fmt.Errorf("DBB_RETENTION_KEEP_DAYS must be >= 0")
	}
	if cfg.RetentionKeepFailedLast < 0 {
		return fmt.Errorf("DBB_RETENTION_KEEP_FAILED_LAST must be >= 0")
	}

	// Validate size fields can be parsed
	if cfg.RetentionMaxTotalSize != "" {
		if _, err := ParseSize(cfg.RetentionMaxTotalSize); err != nil {
			return fmt.Errorf("DBB_RETENTION_MAX_TOTAL_SIZE invalid: %w", err)
		}
	}
	if cfg.DiskMinFreeSize != "" {
		if _, err := ParseSize(cfg.DiskMinFreeSize); err != nil {
			return fmt.Errorf("DBB_DISK_MIN_FREE_SIZE invalid: %w", err)
		}
	}

	// Validate percentage fields
	if cfg.DiskMinFreePercent < 0 || cfg.DiskMinFreePercent > 100 {
		return fmt.Errorf("DBB_DISK_MIN_FREE_PERCENT must be 0-100")
	}
	if cfg.DiskEstimateBufferPercent < 0 || cfg.DiskEstimateBufferPercent > 100 {
		return fmt.Errorf("DBB_DISK_ESTIMATE_BUFFER_PERCENT must be 0-100")
	}

	return nil
}

// validateMySQL validates MySQL configuration
func (v *Validator) validateMySQL(cfg *MySQLConfig) error {
	if len(cfg.Jobs) == 0 {
		return fmt.Errorf("MYSQL_ENABLED=true but MYSQL_JOBS is empty")
	}

	for _, jobName := range cfg.Jobs {
		job, ok := cfg.JobConfigs[jobName]
		if !ok {
			return fmt.Errorf("job %s not found in configuration", jobName)
		}

		if err := v.validateMySQLJob(jobName, &job); err != nil {
			return err
		}
	}

	return nil
}

// validateMySQLJob validates a MySQL job configuration
func (v *Validator) validateMySQLJob(name string, cfg *MySQLJobConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("job %s: MYSQL_%s_HOST is required", name, strings.ToUpper(name))
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("job %s: MYSQL_%s_PORT must be 1-65535", name, strings.ToUpper(name))
	}
	if cfg.User == "" {
		return fmt.Errorf("job %s: MYSQL_%s_USER is required", name, strings.ToUpper(name))
	}
	if cfg.PasswordEnv == "" {
		return fmt.Errorf("job %s: MYSQL_%s_PASSWORD_ENV is required", name, strings.ToUpper(name))
	}
	if len(cfg.Databases) == 0 {
		return fmt.Errorf("job %s: MYSQL_%s_DATABASES is required", name, strings.ToUpper(name))
	}
	if cfg.BackupDir == "" {
		return fmt.Errorf("job %s: MYSQL_%s_BACKUP_DIR is required", name, strings.ToUpper(name))
	}

	// Validate backup dir is absolute path
	if !filepath.IsAbs(cfg.BackupDir) {
		return fmt.Errorf("job %s: MYSQL_%s_BACKUP_DIR must be absolute path", name, strings.ToUpper(name))
	}

	// Validate backup dir is not a dangerous path
	if err := v.validateBackupDir(cfg.BackupDir); err != nil {
		return fmt.Errorf("job %s: %w", name, err)
	}

	// Validate backup mode
	if cfg.BackupMode != "" && cfg.BackupMode != "logical" {
		return fmt.Errorf("job %s: MYSQL_%s_BACKUP_MODE must be logical", name, strings.ToUpper(name))
	}

	// Validate output mode
	if cfg.OutputMode != "" && cfg.OutputMode != "per_database" {
		return fmt.Errorf("job %s: MYSQL_%s_OUTPUT_MODE must be per_database", name, strings.ToUpper(name))
	}

	// Validate set_gtid_purged
	if cfg.SetGtidPurged != "" {
		validGtid := map[string]bool{"OFF": true, "ON": true, "AUTO": true}
		if !validGtid[cfg.SetGtidPurged] {
			return fmt.Errorf("job %s: MYSQL_%s_SET_GTID_PURGED must be OFF, ON, or AUTO", name, strings.ToUpper(name))
		}
	}

	return nil
}

// validatePostgreSQL validates PostgreSQL configuration
func (v *Validator) validatePostgreSQL(cfg *PostgreSQLConfig) error {
	if len(cfg.Jobs) == 0 {
		return fmt.Errorf("POSTGRES_ENABLED=true but POSTGRES_JOBS is empty")
	}

	for _, jobName := range cfg.Jobs {
		job, ok := cfg.JobConfigs[jobName]
		if !ok {
			return fmt.Errorf("job %s not found in configuration", jobName)
		}

		if err := v.validatePostgreSQLJob(jobName, &job); err != nil {
			return err
		}
	}

	return nil
}

// validatePostgreSQLJob validates a PostgreSQL job configuration
func (v *Validator) validatePostgreSQLJob(name string, cfg *PostgreSQLJobConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("job %s: POSTGRES_%s_HOST is required", name, strings.ToUpper(name))
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("job %s: POSTGRES_%s_PORT must be 1-65535", name, strings.ToUpper(name))
	}
	if cfg.User == "" {
		return fmt.Errorf("job %s: POSTGRES_%s_USER is required", name, strings.ToUpper(name))
	}
	if cfg.PasswordEnv == "" {
		return fmt.Errorf("job %s: POSTGRES_%s_PASSWORD_ENV is required", name, strings.ToUpper(name))
	}
	if len(cfg.Databases) == 0 {
		return fmt.Errorf("job %s: POSTGRES_%s_DATABASES is required", name, strings.ToUpper(name))
	}
	if cfg.BackupDir == "" {
		return fmt.Errorf("job %s: POSTGRES_%s_BACKUP_DIR is required", name, strings.ToUpper(name))
	}

	// Validate backup dir is absolute path
	if !filepath.IsAbs(cfg.BackupDir) {
		return fmt.Errorf("job %s: POSTGRES_%s_BACKUP_DIR must be absolute path", name, strings.ToUpper(name))
	}

	// Validate backup dir is not a dangerous path
	if err := v.validateBackupDir(cfg.BackupDir); err != nil {
		return fmt.Errorf("job %s: %w", name, err)
	}

	// Validate dump format
	validFormats := map[string]bool{"custom": true, "plain": true, "directory": true, "tar": true}
	if cfg.DumpFormat != "" && !validFormats[cfg.DumpFormat] {
		return fmt.Errorf("job %s: POSTGRES_%s_DUMP_FORMAT must be custom, plain, directory, or tar", name, strings.ToUpper(name))
	}

	// Validate sslmode
	validSSLModes := map[string]bool{
		"disable": true, "require": true, "verify-ca": true,
		"verify-full": true, "prefer": true, "allow": true,
	}
	if cfg.SSLMode != "" && !validSSLModes[cfg.SSLMode] {
		return fmt.Errorf("job %s: POSTGRES_%s_SSLMODE must be disable, require, verify-ca, verify-full, prefer, or allow", name, strings.ToUpper(name))
	}

	return nil
}

// validateBackupDir validates backup directory is not a dangerous path
func (v *Validator) validateBackupDir(dir string) error {
	if dir == "" {
		return nil
	}

	// Clean the path
	cleaned := filepath.Clean(dir)

	// Check for dangerous paths
	dangerousPaths := []string{"/", "/data", "/tmp", "/var", "/etc", "/usr", "/home", "/root"}
	for _, dangerous := range dangerousPaths {
		if cleaned == dangerous {
			return fmt.Errorf("backup_dir cannot be a dangerous path: %s", dir)
		}
	}

	// Check if path is too short (must have at least 4 levels)
	parts := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	if len(parts) < 3 {
		return fmt.Errorf("backup_dir path is too short: %s (must have at least 3 levels)", dir)
	}

	return nil
}

// ParseSize parses size string to bytes
func ParseSize(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}

	var size int64
	var unit string

	s = strings.TrimSpace(s)
	_, err := fmt.Sscanf(s, "%d%s", &size, &unit)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", s)
	}

	unit = strings.ToUpper(unit)
	switch unit {
	case "G", "GB":
		return size * 1024 * 1024 * 1024, nil
	case "M", "MB":
		return size * 1024 * 1024, nil
	case "K", "KB":
		return size * 1024, nil
	case "B", "":
		return size, nil
	default:
		return 0, fmt.Errorf("unknown size unit: %s", unit)
	}
}