package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// FileSHA256 calculates SHA256 checksum of a file
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyFileSHA256 verifies file checksum against expected value
func VerifyFileSHA256(path, expected string) (bool, error) {
	actual, err := FileSHA256(path)
	if err != nil {
		return false, err
	}
	return actual == expected, nil
}