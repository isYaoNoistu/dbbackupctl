//go:build windows

package disk

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// getDiskUsagePlatform returns disk usage for Windows
func getDiskUsagePlatform(path string) (*DiskUsage, error) {
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
	if err != nil {
		return nil, err
	}

	return &DiskUsage{
		Total:     int64(totalBytes),
		Free:      int64(freeBytesAvailable),
		Available: int64(totalFreeBytes),
		Used:      int64(totalBytes - freeBytesAvailable),
	}, nil
}

func init() {
	// Suppress unused import warning
	_ = unsafe.Pointer(nil)
}