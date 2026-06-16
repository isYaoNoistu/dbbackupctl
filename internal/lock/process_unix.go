//go:build !windows

package lock

import "syscall"

// isProcessRunning checks if a process is still running
// Uses signal 0 to check process existence without sending any signal
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}