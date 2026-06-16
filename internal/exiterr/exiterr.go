package exiterr

import "fmt"

// Exit codes for dbbackupctl
const (
	ExitOK               = 0
	ExitGeneral          = 1
	ExitConfig           = 2
	ExitDependency       = 3
	ExitDBConnection     = 4
	ExitBackupFailed     = 5
	ExitCompressFailed   = 6
	ExitDiskInsufficient = 7
	ExitChecksumFailed   = 8
	ExitPruneFailed      = 9
	ExitLockConflict     = 10
	ExitRestoreFailed    = 11
	ExitIndexError       = 12
	ExitPermission       = 13
)

// ExitError represents an error with an exit code
type ExitError struct {
	Code int
	Err  error
}

// Error returns the error message
func (e *ExitError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the wrapped error
func (e *ExitError) Unwrap() error {
	return e.Err
}

// New creates a new ExitError
func New(code int, err error) *ExitError {
	return &ExitError{
		Code: code,
		Err:  err,
	}
}

// Newf creates a new ExitError with formatted message
func Newf(code int, format string, args ...interface{}) *ExitError {
	return &ExitError{
		Code: code,
		Err:  fmt.Errorf(format, args...),
	}
}
