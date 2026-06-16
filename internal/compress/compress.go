package compress

import (
	"fmt"
	"os"
	"os/exec"
)

// CompressionType represents compression type
type CompressionType string

const (
	CompressionZstd CompressionType = "zstd"
	CompressionGzip CompressionType = "gzip"
	CompressionNone CompressionType = "none"
)

// Compressor handles file compression
type Compressor struct {
	Type  CompressionType
	Level int
}

// NewCompressor creates a new compressor
func NewCompressor(compressType string, level int) *Compressor {
	switch compressType {
	case "zstd":
		return &Compressor{Type: CompressionZstd, Level: level}
	case "gzip":
		return &Compressor{Type: CompressionGzip, Level: level}
	default:
		return &Compressor{Type: CompressionNone, Level: 0}
	}
}

// Extension returns the file extension for the compression type
func (c *Compressor) Extension() string {
	switch c.Type {
	case CompressionZstd:
		return ".zst"
	case CompressionGzip:
		return ".gz"
	default:
		return ""
	}
}

// CompressCommand returns the compression command
func (c *Compressor) CompressCommand() (string, []string) {
	switch c.Type {
	case CompressionZstd:
		level := c.Level
		if level == 0 {
			level = 3
		}
		return "zstd", []string{"-", fmt.Sprintf("-%d", level)}
	case CompressionGzip:
		level := c.Level
		if level == 0 {
			level = 6
		}
		return "gzip", []string{"-", fmt.Sprintf("-%d", level)}
	default:
		return "cat", nil
	}
}

// DecompressCommand returns the decompression command
func (c *Compressor) DecompressCommand(filePath string) (string, []string) {
	switch c.Type {
	case CompressionZstd:
		return "zstd", []string{"-dc", filePath}
	case CompressionGzip:
		return "gzip", []string{"-dc", filePath}
	default:
		return "cat", []string{filePath}
	}
}

// CompressFile compresses a file
func (c *Compressor) CompressFile(inputPath, outputPath string) error {
	if c.Type == CompressionNone {
		// Just copy the file
		return copyFile(inputPath, outputPath)
	}

	cmdName, args := c.CompressCommand()
	cmd := exec.Command(cmdName, args...)
	cmd.Args = append(cmd.Args, "-o", outputPath, inputPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compression failed: %s", string(output))
	}

	return nil
}

// DecompressFile decompresses a file
func (c *Compressor) DecompressFile(inputPath, outputPath string) error {
	if c.Type == CompressionNone {
		return copyFile(inputPath, outputPath)
	}

	cmdName, args := c.DecompressCommand(inputPath)
	cmd := exec.Command(cmdName, args...)

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("decompression failed: %s", string(output))
	}

	return nil
}

// copyFile copies a file
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}