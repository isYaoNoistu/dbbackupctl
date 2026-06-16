package configenv

// CoreConfig holds core configuration
type CoreConfig struct {
	// Directory configuration
	ConfigDir  string
	DataDir    string
	BackupRoot string
	TmpDir     string
	LogDir     string
	LockDir    string

	// General configuration
	Timezone       string
	LogFormat      string
	LogLevel       string
	DefaultTimeout string

	// Compression configuration
	CompressEnabled bool
	CompressType    string
	CompressLevel   int
	CompressThreads int
	StreamCompress  bool

	// Checksum configuration
	ChecksumEnabled bool
	ChecksumType    string

	// Retention policy configuration
	RetentionKeepLast          int
	RetentionKeepDays          int
	RetentionKeepFailedLast    int
	RetentionMaxTotalSize      string
	RetentionPruneBeforeBackup bool
	RetentionPruneAfterBackup  bool

	// Disk protection configuration
	DiskGuardEnabled          bool
	DiskMinFreeSize           string
	DiskMinFreePercent        int
	DiskEstimateBufferPercent int

	// Index configuration
	IndexFile      string
	CommandLogFile string
	RestoreLogFile string
}
