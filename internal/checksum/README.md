# Package `checksum` (`internal/checksum`)

`internal/checksum` provides cryptographic hashing and integrity verification utilities for downloaded files.

## Purpose

The package is responsible for:
- Computing SHA-256 hashes of local files using memory-efficient buffered streaming (64 KB buffers).
- Verifying downloaded files against expected SHA-256 hex digests with case-insensitive matching.
- Facilitating post-download integrity validation before renaming temporary `.part` files to final destination files.

## Key Functions

- [`ComputeFileSHA256(filePath string) (string, error)`](file:///D:/dev/downloader/internal/checksum/checksum.go#L14): Computes the hex-encoded SHA-256 digest of a target file.
- [`VerifyFile(filePath, expectedHex string) (bool, string, error)`](file:///D:/dev/downloader/internal/checksum/checksum.go#L30): Compares file SHA-256 digest against an expected hex string; safely skips check if no expected hash is provided.
