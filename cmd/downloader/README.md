# Package `main` (`cmd/downloader`)

`cmd/downloader` serves as the CLI entry point for the Go Concurrent Resumable Downloader application.

## Purpose

The package is responsible for:
- Parsing command-line flags and routing subcommands (`download`, `resume`, `list`, `status`).
- Reading and parsing batch URL input files (with support for comments and optional SHA-256 checksums).
- Converting user-specified bandwidth strings (such as `5MB/s` or `500KB/s`) into raw bytes per second.
- Initializing the local session store and orchestrating the download engine.

## Subcommands

- **Default / Download**: Starts a new download session from a URL list file.
- **`resume <session-name>`**: Reconnects and resumes pending or interrupted downloads from an existing session.
- **`list`**: Summarizes all recorded sessions stored in `~/.downloader/sessions`.
- **`status <session-name>`**: Displays per-file status, downloaded bytes, and retry counts for a given session.
