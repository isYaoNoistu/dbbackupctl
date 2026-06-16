package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Writer handles manifest file writing
type Writer struct{}

// NewWriter creates a new manifest writer
func NewWriter() *Writer {
	return &Writer{}
}

// Write writes a manifest to a file
func (w *Writer) Write(m *Manifest, dir string) error {
	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 manifest 失败: %w", err)
	}

	// Write to file
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入 manifest 失败: %w", err)
	}

	return nil
}

// Read reads a manifest from a file
func (w *Writer) Read(dir string) (*Manifest, error) {
	path := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 manifest 失败: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败: %w", err)
	}

	return &m, nil
}
