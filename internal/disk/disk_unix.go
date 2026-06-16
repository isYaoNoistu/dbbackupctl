//go:build !windows

package disk

import "syscall"

// getDiskUsagePlatform returns disk usage for Unix/Linux
func getDiskUsagePlatform(path string) (*DiskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, err
	}

	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	available := int64(stat.Bfree) * int64(stat.Bsize)
	used := total - free

	return &DiskUsage{
		Total:     total,
		Free:      free,
		Available: available,
		Used:      used,
	}, nil
}
