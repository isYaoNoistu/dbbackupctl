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
		return fmt.Errorf("核心配置错误: %w", err)
	}

	if cfg.MySQL.Enabled {
		if err := v.validateMySQL(&cfg.MySQL); err != nil {
			return fmt.Errorf("MySQL 配置错误: %w", err)
		}
	}

	if cfg.PostgreSQL.Enabled {
		if err := v.validatePostgreSQL(&cfg.PostgreSQL); err != nil {
			return fmt.Errorf("PostgreSQL 配置错误: %w", err)
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
			return fmt.Errorf("%s 必须是绝对路径: %s", name, dir)
		}
	}

	// Validate backup root is not a dangerous path
	if err := v.validateBackupDir(cfg.BackupRoot); err != nil {
		return fmt.Errorf("DBB_BACKUP_ROOT: %w", err)
	}

	// Validate compress type
	validCompressTypes := map[string]bool{"zstd": true, "gzip": true, "none": true}
	if cfg.CompressType != "" && !validCompressTypes[cfg.CompressType] {
		return fmt.Errorf("DBB_COMPRESS_TYPE 非法: %s（必须是 zstd、gzip 或 none）", cfg.CompressType)
	}

	// Validate checksum type
	if cfg.ChecksumType != "" && cfg.ChecksumType != "sha256" {
		return fmt.Errorf("DBB_CHECKSUM_TYPE 非法: %s（必须是 sha256）", cfg.ChecksumType)
	}

	// Validate compress level
	if cfg.CompressType == "zstd" && cfg.CompressLevel != 0 && (cfg.CompressLevel < 1 || cfg.CompressLevel > 22) {
		return fmt.Errorf("DBB_COMPRESS_LEVEL 对 zstd 非法: %d（必须是 1-22）", cfg.CompressLevel)
	}
	if cfg.CompressType == "gzip" && cfg.CompressLevel != 0 && (cfg.CompressLevel < 1 || cfg.CompressLevel > 9) {
		return fmt.Errorf("DBB_COMPRESS_LEVEL 对 gzip 非法: %d（必须是 1-9）", cfg.CompressLevel)
	}

	// Validate retention settings
	if cfg.RetentionKeepLast < 0 {
		return fmt.Errorf("DBB_RETENTION_KEEP_LAST 必须大于等于 0")
	}
	if cfg.RetentionKeepDays < 0 {
		return fmt.Errorf("DBB_RETENTION_KEEP_DAYS 必须大于等于 0")
	}
	if cfg.RetentionKeepFailedLast < 0 {
		return fmt.Errorf("DBB_RETENTION_KEEP_FAILED_LAST 必须大于等于 0")
	}

	// Validate size fields can be parsed
	if cfg.RetentionMaxTotalSize != "" {
		if _, err := ParseSize(cfg.RetentionMaxTotalSize); err != nil {
			return fmt.Errorf("DBB_RETENTION_MAX_TOTAL_SIZE 非法: %w", err)
		}
	}
	if cfg.DiskMinFreeSize != "" {
		if _, err := ParseSize(cfg.DiskMinFreeSize); err != nil {
			return fmt.Errorf("DBB_DISK_MIN_FREE_SIZE 非法: %w", err)
		}
	}

	// Validate percentage fields
	if cfg.DiskMinFreePercent < 0 || cfg.DiskMinFreePercent > 100 {
		return fmt.Errorf("DBB_DISK_MIN_FREE_PERCENT 必须在 0-100 之间")
	}
	if cfg.DiskEstimateBufferPercent < 0 || cfg.DiskEstimateBufferPercent > 100 {
		return fmt.Errorf("DBB_DISK_ESTIMATE_BUFFER_PERCENT 必须在 0-100 之间")
	}

	return nil
}

// validateMySQL validates MySQL configuration
func (v *Validator) validateMySQL(cfg *MySQLConfig) error {
	if len(cfg.Jobs) == 0 {
		return fmt.Errorf("MYSQL_ENABLED=true 但 MYSQL_JOBS 为空")
	}

	for _, jobName := range cfg.Jobs {
		job, ok := cfg.JobConfigs[jobName]
		if !ok {
			return fmt.Errorf("配置中未找到环境 %s", jobName)
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
		return fmt.Errorf("环境 %s: MYSQL_%s_HOST 必填", name, strings.ToUpper(name))
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("环境 %s: MYSQL_%s_PORT 必须在 1-65535 之间", name, strings.ToUpper(name))
	}
	if cfg.User == "" {
		return fmt.Errorf("环境 %s: MYSQL_%s_USER 必填", name, strings.ToUpper(name))
	}
	if cfg.PasswordEnv == "" {
		return fmt.Errorf("环境 %s: MYSQL_%s_PASSWORD_ENV 必填", name, strings.ToUpper(name))
	}
	if len(cfg.Databases) == 0 {
		return fmt.Errorf("环境 %s: MYSQL_%s_DATABASES 必填", name, strings.ToUpper(name))
	}
	if cfg.BackupDir == "" {
		return fmt.Errorf("环境 %s: MYSQL_%s_BACKUP_DIR 必填", name, strings.ToUpper(name))
	}

	// Validate backup dir is absolute path
	if !filepath.IsAbs(cfg.BackupDir) {
		return fmt.Errorf("环境 %s: MYSQL_%s_BACKUP_DIR 必须是绝对路径", name, strings.ToUpper(name))
	}

	// Validate backup dir is not a dangerous path
	if err := v.validateBackupDir(cfg.BackupDir); err != nil {
		return fmt.Errorf("环境 %s: %w", name, err)
	}

	// Validate backup mode
	if cfg.BackupMode != "" && cfg.BackupMode != "logical" {
		return fmt.Errorf("环境 %s: MYSQL_%s_BACKUP_MODE 必须是 logical", name, strings.ToUpper(name))
	}

	// Validate output mode
	if cfg.OutputMode != "" && cfg.OutputMode != "per_database" {
		return fmt.Errorf("环境 %s: MYSQL_%s_OUTPUT_MODE 必须是 per_database", name, strings.ToUpper(name))
	}

	// Validate set_gtid_purged
	if cfg.SetGtidPurged != "" {
		validGtid := map[string]bool{"OFF": true, "ON": true, "AUTO": true}
		if !validGtid[cfg.SetGtidPurged] {
			return fmt.Errorf("环境 %s: MYSQL_%s_SET_GTID_PURGED 必须是 OFF、ON 或 AUTO", name, strings.ToUpper(name))
		}
	}

	return nil
}

// validatePostgreSQL validates PostgreSQL configuration
func (v *Validator) validatePostgreSQL(cfg *PostgreSQLConfig) error {
	if len(cfg.Jobs) == 0 {
		return fmt.Errorf("POSTGRES_ENABLED=true 但 POSTGRES_JOBS 为空")
	}

	for _, jobName := range cfg.Jobs {
		job, ok := cfg.JobConfigs[jobName]
		if !ok {
			return fmt.Errorf("配置中未找到环境 %s", jobName)
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
		return fmt.Errorf("环境 %s: POSTGRES_%s_HOST 必填", name, strings.ToUpper(name))
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("环境 %s: POSTGRES_%s_PORT 必须在 1-65535 之间", name, strings.ToUpper(name))
	}
	if cfg.User == "" {
		return fmt.Errorf("环境 %s: POSTGRES_%s_USER 必填", name, strings.ToUpper(name))
	}
	if cfg.PasswordEnv == "" {
		return fmt.Errorf("环境 %s: POSTGRES_%s_PASSWORD_ENV 必填", name, strings.ToUpper(name))
	}
	if len(cfg.Databases) == 0 {
		return fmt.Errorf("环境 %s: POSTGRES_%s_DATABASES 必填", name, strings.ToUpper(name))
	}
	if cfg.BackupDir == "" {
		return fmt.Errorf("环境 %s: POSTGRES_%s_BACKUP_DIR 必填", name, strings.ToUpper(name))
	}

	// Validate backup dir is absolute path
	if !filepath.IsAbs(cfg.BackupDir) {
		return fmt.Errorf("环境 %s: POSTGRES_%s_BACKUP_DIR 必须是绝对路径", name, strings.ToUpper(name))
	}

	// Validate backup dir is not a dangerous path
	if err := v.validateBackupDir(cfg.BackupDir); err != nil {
		return fmt.Errorf("环境 %s: %w", name, err)
	}

	// Validate dump format
	validFormats := map[string]bool{"custom": true, "plain": true, "tar": true}
	if cfg.DumpFormat != "" && !validFormats[cfg.DumpFormat] {
		if cfg.DumpFormat == "directory" {
			return fmt.Errorf("环境 %s: v1 流式模式不支持 directory 格式", name)
		}
		return fmt.Errorf("环境 %s: POSTGRES_%s_DUMP_FORMAT 必须是 custom、plain 或 tar", name, strings.ToUpper(name))
	}

	// Validate sslmode
	validSSLModes := map[string]bool{
		"disable": true, "require": true, "verify-ca": true,
		"verify-full": true, "prefer": true, "allow": true,
	}
	if cfg.SSLMode != "" && !validSSLModes[cfg.SSLMode] {
		return fmt.Errorf("环境 %s: POSTGRES_%s_SSLMODE 必须是 disable、require、verify-ca、verify-full、prefer 或 allow", name, strings.ToUpper(name))
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
			return fmt.Errorf("backup_dir 不能是危险路径: %s", dir)
		}
	}

	// Check if path is too short (must have at least 4 levels)
	parts := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	if len(parts) < 3 {
		return fmt.Errorf("backup_dir 路径过短: %s（至少需要 3 级目录）", dir)
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
		return 0, fmt.Errorf("大小格式非法: %s", s)
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
		return 0, fmt.Errorf("未知大小单位: %s", unit)
	}
}
