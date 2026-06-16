package index

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/isYaoNoistu/dbbackupctl/internal/manifest"
)

// Store handles backup record storage using JSONL format
type Store struct {
	filePath string
	mu       sync.Mutex
}

// NewStore creates a new index store
func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
	}
}

// Append adds a backup record to the index
func (s *Store) Append(record BackupRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建索引目录失败: %w", err)
	}

	// Open file in append mode
	f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开索引文件失败: %w", err)
	}
	defer f.Close()

	// Marshal record to JSON
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("序列化记录失败: %w", err)
	}

	// Write record with newline
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("写入记录失败: %w", err)
	}

	return nil
}

// Query queries backup records from the index
func (s *Store) Query(filter QueryFilter) ([]BackupRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Open file
	f, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("打开索引文件失败: %w", err)
	}
	defer f.Close()

	var records []BackupRecord
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var record BackupRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue // Skip invalid records
		}

		// Apply filters
		if filter.DBType != "" && record.DBType != filter.DBType {
			continue
		}
		if filter.Job != "" && record.Job != filter.Job {
			continue
		}

		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取索引文件失败: %w", err)
	}

	// Apply limit (from the end, most recent first)
	if filter.Limit > 0 && len(records) > filter.Limit {
		records = records[len(records)-filter.Limit:]
	}

	// Reverse to show most recent first
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// FindByID finds a backup record by ID
func (s *Store) FindByID(backupID string) (*BackupRecord, error) {
	records, err := s.Query(QueryFilter{})
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		if r.BackupID == backupID {
			return &r, nil
		}
	}

	return nil, fmt.Errorf("未找到备份记录: %s", backupID)
}

// Rebuild rebuilds the index from backup directories
func (s *Store) Rebuild(root string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if root == "" {
		return fmt.Errorf("备份根目录必填")
	}

	var records []BackupRecord
	mw := manifest.NewWriter()

	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info == nil || info.IsDir() || info.Name() != "manifest.json" {
			return nil
		}

		backupDir := filepath.Dir(path)
		m, err := mw.Read(backupDir)
		if err != nil {
			return nil
		}
		if m.BackupID == "" || m.DBType == "" || m.Job == "" || m.Status == "" {
			return nil
		}

		recordBackupDir := m.BackupDir
		if recordBackupDir == "" {
			recordBackupDir = backupDir
		}

		var totalSize int64
		for _, a := range m.Artifacts {
			totalSize += a.SizeBytes
		}

		records = append(records, BackupRecord{
			BackupID:    m.BackupID,
			DBType:      m.DBType,
			Job:         m.Job,
			Status:      m.Status,
			StartedAt:   m.StartedAt,
			DurationSec: m.DurationSec,
			SizeBytes:   totalSize,
			BackupDir:   recordBackupDir,
			Manifest:    path,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("扫描备份根目录失败: %w", err)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt.Before(records[j].StartedAt)
	})

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return fmt.Errorf("创建索引目录失败: %w", err)
	}

	tmpPath := s.filePath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建临时索引失败: %w", err)
	}

	enc := json.NewEncoder(f)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			f.Close()
			return fmt.Errorf("写入临时索引失败: %w", err)
		}
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("同步临时索引失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("关闭临时索引失败: %w", err)
	}

	if _, err := os.Stat(s.filePath); err == nil {
		backupPath := fmt.Sprintf("%s.bak.%s", s.filePath, time.Now().Format("20060102-150405"))
		if err := copyFile(s.filePath, backupPath); err != nil {
			return fmt.Errorf("备份已有索引失败: %w", err)
		}
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("替换索引失败: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
