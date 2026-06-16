//go:build windows

package lock

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// isProcessRunning checks if a process is still running on Windows
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Open process with minimal access rights
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	// Check if process is still active
	var exitCode uint32
	err = windows.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}

	// If exit code is still active, process is running
	return exitCode == 259 // STILL_ACTIVE
}

func init() {
	// Suppress unused import warning
	_ = unsafe.Pointer(nil)
}
