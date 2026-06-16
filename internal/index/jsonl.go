package index

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
		return fmt.Errorf("creating index directory: %w", err)
	}

	// Open file in append mode
	f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening index file: %w", err)
	}
	defer f.Close()

	// Marshal record to JSON
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshaling record: %w", err)
	}

	// Write record with newline
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing record: %w", err)
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
		return nil, fmt.Errorf("opening index file: %w", err)
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
		return nil, fmt.Errorf("reading index file: %w", err)
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

	return nil, fmt.Errorf("backup record not found: %s", backupID)
}

// Rebuild rebuilds the index from backup directories
func (s *Store) Rebuild(root string) error {
	// TODO: Implement rebuild logic
	return fmt.Errorf("not implemented")
}
