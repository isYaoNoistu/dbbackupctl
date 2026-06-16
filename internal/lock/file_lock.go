package lock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LockInfo holds lock information
type LockInfo struct {
	PID       int    `json:"pid"`
	DBType    string `json:"db_type"`
	Job       string `json:"job"`
	StartedAt string `json:"started_at"`
}

// Manager handles file locking
type Manager struct {
	lockDir string
}

// NewManager creates a new lock manager
func NewManager(lockDir string) *Manager {
	return &Manager{
		lockDir: lockDir,
	}
}

// Acquire acquires a lock for a job
func (m *Manager) Acquire(dbType, job string, force bool) error {
	lockFile := m.lockFilePath(dbType, job)

	// Check if lock exists
	if _, err := os.Stat(lockFile); err == nil {
		// Lock exists, check if it's stale
		lockInfo, err := m.readLock(lockFile)
		if err != nil {
			// Can't read lock, consider it stale
			if force {
				return m.removeLock(lockFile)
			}
			return fmt.Errorf("锁文件已存在但无法读取: %s", lockFile)
		}

		// Check if process is still running
		if isProcessRunning(lockInfo.PID) {
			return fmt.Errorf("锁冲突: 任务 %s/%s 正在运行（PID: %d，启动时间: %s）",
				dbType, job, lockInfo.PID, lockInfo.StartedAt)
		}

		// Process is not running, lock is stale
		if force {
			if err := m.removeLock(lockFile); err != nil {
				return fmt.Errorf("删除陈旧锁失败: %w", err)
			}
		} else {
			return fmt.Errorf("发现陈旧锁（PID %d 未运行），请使用 --force 清理", lockInfo.PID)
		}
	}

	// Create lock directory if it doesn't exist
	if err := os.MkdirAll(m.lockDir, 0755); err != nil {
		return fmt.Errorf("创建锁目录失败: %w", err)
	}

	// Write lock file
	lockInfo := LockInfo{
		PID:       os.Getpid(),
		DBType:    dbType,
		Job:       job,
		StartedAt: time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(lockInfo)
	if err != nil {
		return fmt.Errorf("序列化锁信息失败: %w", err)
	}

	if err := os.WriteFile(lockFile, data, 0640); err != nil {
		return fmt.Errorf("写入锁文件失败: %w", err)
	}

	return nil
}

// Release releases a lock for a job
func (m *Manager) Release(dbType, job string) error {
	lockFile := m.lockFilePath(dbType, job)

	// Check if lock exists
	if _, err := os.Stat(lockFile); os.IsNotExist(err) {
		return nil // Lock doesn't exist
	}

	// Read lock to verify it's ours
	lockInfo, err := m.readLock(lockFile)
	if err != nil {
		// Can't read lock, try to remove it
		return m.removeLock(lockFile)
	}

	// Check if it's our lock
	if lockInfo.PID != os.Getpid() {
		return fmt.Errorf("锁属于其他进程（PID: %d）", lockInfo.PID)
	}

	return m.removeLock(lockFile)
}

// lockFilePath returns the path to the lock file
func (m *Manager) lockFilePath(dbType, job string) string {
	return filepath.Join(m.lockDir, fmt.Sprintf("%s-%s.lock", dbType, job))
}

// readLock reads lock information from a file
func (m *Manager) readLock(lockFile string) (*LockInfo, error) {
	data, err := os.ReadFile(lockFile)
	if err != nil {
		return nil, err
	}

	var lockInfo LockInfo
	if err := json.Unmarshal(data, &lockInfo); err != nil {
		return nil, err
	}

	return &lockInfo, nil
}

// removeLock removes a lock file
func (m *Manager) removeLock(lockFile string) error {
	return os.Remove(lockFile)
}
