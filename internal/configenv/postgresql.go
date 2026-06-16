package configenv

// PostgreSQLConfig holds PostgreSQL configuration
type PostgreSQLConfig struct {
	Enabled    bool
	Jobs       []string
	JobConfigs map[string]PostgreSQLJobConfig
}

// PostgreSQLJobConfig holds PostgreSQL job configuration
type PostgreSQLJobConfig struct {
	// Connection
	Enabled      bool
	Host         string
	Port         int
	User         string
	PasswordEnv  string
	PasswordFile string
	SSLMode      string

	// Database selection
	Databases                []string
	IncludeTemplateDatabases bool
	IncludePostgresDatabase  bool

	// Backup settings
	BackupMode     string
	DumpFormat     string
	IncludeGlobals bool
	NoOwner        bool
	NoPrivileges   bool
	Jobs           int
	BackupDir      string

	// Restore settings
	RestoreHost         string
	RestorePort         int
	RestoreUser         string
	RestorePasswordEnv  string
	RestorePasswordFile string
	RestoreSSLMode      string

	// Retention policy
	RetentionKeepLast     int
	RetentionKeepDays     int
	RetentionMaxTotalSize string
}
