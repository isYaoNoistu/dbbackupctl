package disk

import (
	"fmt"
	"os"
	"path/filepath"
)

// Guard handles disk space protection
type Guard struct {
	minFreeSize    int64
	minFreePercent int
	estimateBuffer int
}

// NewGuard creates a new disk guard
func NewGuard(minFreeSize int64, minFreePercent, estimateBuffer int) *Guard {
	return &Guard{
		minFreeSize:    minFreeSize,
		minFreePercent: minFreePercent,
		estimateBuffer: estimateBuffer,
	}
}

// CheckDiskSpace checks if there's enough disk space for backup
func (g *Guard) CheckDiskSpace(backupDir string, estimatedSize int64) error {
	// Get disk usage
	usage, err := GetDiskUsage(backupDir)
	if err != nil {
		return fmt.Errorf("getting disk usage: %w", err)
	}

	// Check minimum free space
	if usage.Free < g.minFreeSize {
		return fmt.Errorf("insufficient disk space: free %s, required %s",
			FormatBytes(usage.Free), FormatBytes(g.minFreeSize))
	}

	// Check free percentage
	freePercent := float64(usage.Free) / float64(usage.Total) * 100
	if freePercent < float64(g.minFreePercent) {
		return fmt.Errorf("insufficient disk space: %.1f%% free, required %d%%",
			freePercent, g.minFreePercent)
	}

	// Check estimated size with buffer
	requiredSize := estimatedSize * int64(100+g.estimateBuffer) / 100
	if usage.Free < requiredSize {
		return fmt.Errorf("insufficient disk space for backup: free %s, estimated required %s (with %d%% buffer)",
			FormatBytes(usage.Free), FormatBytes(requiredSize), g.estimateBuffer)
	}

	return nil
}

// DiskUsage holds disk usage information
type DiskUsage struct {
	Total     int64
	Free      int64
	Available int64
	Used      int64
}

// GetDiskUsage returns disk usage for a path
func GetDiskUsage(path string) (*DiskUsage, error) {
	// Ensure path exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Try parent directory
		dir = filepath.Dir(dir)
	}

	return getDiskUsagePlatform(dir)
}

// FormatBytes formats bytes to human readable string
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}