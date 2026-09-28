# Package `checksum` (`internal/checksum`)

`internal/checksum` provides SHA-256 hash generation and verification utilities for validating file integrity upon download completion.

## Purpose

The package calculates SHA-256 digests using streaming algorithms without loading entire large files into memory. It ensures that files fetched over the network match expected hashes provided in URL files or command-line arguments.

## Key Functions

- [`ComputeFileSHA256(filePath string) (string, error)`](checksum.go#L14): Computes the hex-encoded SHA-256 digest of a target file.
- [`VerifyFile(filePath, expectedHex string) (bool, string, error)`](checksum.go#L30): Compares file SHA-256 digest against an expected hex string; safely skips check if no expected hash is provided.
