package configenv

// MySQLConfig holds MySQL configuration
type MySQLConfig struct {
	Enabled    bool
	Jobs       []string
	JobConfigs map[string]MySQLJobConfig
}

// MySQLJobConfig holds MySQL job configuration
type MySQLJobConfig struct {
	// Connection
	Enabled      bool
	Host         string
	Port         int
	User         string
	PasswordEnv  string
	PasswordFile string

	// Database selection
	Databases              []string
	IncludeSystemDatabases bool

	// Backup settings
	BackupMode         string
	OutputMode         string
	BackupDir          string
	SingleTransaction  bool
	Quick              bool
	Routines           bool
	Events             bool
	Triggers           bool
	HexBlob            bool
	SetGtidPurged      string
	ColumnStatistics   bool
	LockTables         bool
	DumpCreateDatabase bool

	// Restore settings
	RestoreHost         string
	RestorePort         int
	RestoreUser         string
	RestorePasswordEnv  string
	RestorePasswordFile string

	// Retention policy
	RetentionKeepLast     int
	RetentionKeepDays     int
	RetentionMaxTotalSize string
}
