// Package checksum provides cryptographic hashing and integrity verification utilities.
package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// ComputeFileSHA256 calculates the hex-encoded SHA-256 hash of the specified file using a 64KB buffer.
func ComputeFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(hasher, f, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// VerifyFile compares the SHA-256 hash of filePath against expectedHex.
// If expectedHex is empty, verification is skipped and returns true.
// The comparison is case-insensitive.
func VerifyFile(filePath, expectedHex string) (bool, string, error) {
	if expectedHex == "" {
		return true, "", nil
	}
	actual, err := ComputeFileSHA256(filePath)
	if err != nil {
		return false, "", fmt.Errorf("failed computing sha256: %w", err)
	}
	match := strings.EqualFold(actual, strings.TrimSpace(expectedHex))
	return match, actual, nil
}
