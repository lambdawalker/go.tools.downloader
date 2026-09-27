# Go Concurrent Resumable Downloader

A high-performance, resilient command-line download manager built in Go. It features concurrent multi-worker streaming, pre-flight metadata probing, Shortest-Job-First (SJF) queue scheduling, dynamic stall and slow-stream demotion watchdog, token-bucket bandwidth limiting, session persistence, and real-time ANSI terminal dashboard visualization.

---

## Features

- **Concurrent Multi-Worker Engine**: Configurable pool of concurrent worker goroutines downloading files simultaneously.
- **Pre-Flight Metadata Probing**: Discovers file size, HTTP byte-range capability, cache validators (`ETag`, `Last-Modified`), and canonical file names via HTTP `HEAD` or fallback `Range: bytes=0-0` requests.
- **Shortest-Job-First (SJF) Scheduling**: Automatically downloads smaller files first to minimize average waiting time and complete jobs faster, while placing unknown-size files at the end.
- **Intelligent Watchdog & Stream Demotion**: Continuously evaluates stream health and peer transfer speeds. Resumable streams that stall (15s inactivity) or run chronically slower than peer averages (<2/3 peer average) are automatically paused, demoted to the back of the queue, and replaced by queued downloads.
- **Resumable Partial Transfers**: Automatically detects `.part` files on disk and resumes transfers using HTTP `Range: bytes=<offset>-` requests and conditional `If-Range` headers to guard against modified content.
- **Token-Bucket Bandwidth Limiter**: Throttle global download throughput using human-friendly rate limit strings (e.g. `10MB/s`, `500KB/s`).
- **Integrity Verification**: Automatic SHA-256 checksum verification before promoting `.part` files to final destination files.
- **Atomic File Promotion**: Downloads are streamed to temporary `.part` files and atomically renamed only upon complete and verified download.
- **Persistent Sessions**: State is automatically recorded to `~/.downloader/sessions/<name>.json`, allowing full recovery and session inspection across interruptions or terminal crashes.
- **Live Terminal Dashboard**: Interactive multi-line ANSI dashboard displaying worker states, progress bars, throughput gauges, and stream statuses (`WARMING_UP`, `STABLE`, `STALLED`). Gracefully falls back to clean interval logs in non-interactive environments.

---

## Project Structure

| Package | Path | Purpose |
| :--- | :--- | :--- |
| **`main`** | [`cmd/downloader`](file:///D:/dev/downloader/cmd/downloader/README.md) | CLI entry point, flag parsing, and subcommand dispatching. |
| **`checksum`** | [`internal/checksum`](file:///D:/dev/downloader/internal/checksum/README.md) | SHA-256 hash generation and integrity verification. |
| **`dashboard`** | [`internal/dashboard`](file:///D:/dev/downloader/internal/dashboard/README.md) | Interactive ANSI terminal dashboard and headless progress logging. |
| **`engine`** | [`internal/engine`](file:///D:/dev/downloader/internal/engine/README.md) | Central download orchestrator connecting workers, queue, watchdog, and store. |
| **`limiter`** | [`internal/limiter`](file:///D:/dev/downloader/internal/limiter/README.md) | Token-bucket rate limiter and throttled `io.Reader`. |
| **`model`** | [`internal/model`](file:///D:/dev/downloader/internal/model/README.md) | Domain types, session representations, status enums, and byte/speed formatters. |
| **`prober`** | [`internal/prober`](file:///D:/dev/downloader/internal/prober/README.md) | HTTP pre-flight metadata prober, range detector, and filename sanitizer. |
| **`queue`** | [`internal/queue`](file:///D:/dev/downloader/internal/queue/README.md) | Synchronized job queue with SJF prioritization and demotion mechanics. |
| **`store`** | [`internal/store`](file:///D:/dev/downloader/internal/store/README.md) | Atomic JSON persistence for session files in `~/.downloader/sessions`. |
| **`transfer`** | [`internal/transfer`](file:///D:/dev/downloader/internal/transfer/README.md) | Resumable HTTP download streamer, `.part` file manager, and sliding-window speed tracker. |
| **`watchdog`** | [`internal/watchdog`](file:///D:/dev/downloader/internal/watchdog/README.md) | Active stream monitor, stall detector, and adaptive demotion policy engine. |

---

## Installation & Building

### Prerequisites
- [Go 1.21+](https://go.dev/dl/) installed and available in your `PATH`.

### Build Commands

```bash
# Direct Go build
go build -trimpath -ldflags="-s -w" -o bin/downloader.exe ./cmd/downloader

# Using Makefile
make build          # Builds for current host OS/architecture
make windows        # Builds Windows binaries in bin/
make linux          # Builds Linux binaries in bin/
make all            # Builds for both Linux and Windows

# Using Platform Scripts
.\build.ps1         # Windows PowerShell
.\build.bat         # Windows Command Prompt
./build.sh          # Linux / macOS shell script
```

---

## Input File Format

The downloader accepts a plain text file containing a list of URLs to download, with support for comments (`#`), blank lines, and optional expected SHA-256 checksums separated by whitespace.

Example `urls.txt`:
```text
# Linux ISOs and Distribution Images
https://releases.ubuntu.com/22.04.4/ubuntu-22.04.4-desktop-amd64.iso
https://deb.debian.org/debian/dists/bookworm/main/installer-amd64/current/images/netboot/mini.iso

# URL with expected SHA-256 checksum
https://example.com/data/archive.tar.gz 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
```

---

## Usage & Examples

### 1. Basic Batch Download
Download all URLs listed in `urls.txt` into the current working directory:
```bash
downloader -file urls.txt
```

### 2. Custom Output Directory & Session Name
Download files into a specific folder (`./downloads`) and give the session a memorable name (`my_backup`):
```bash
downloader -file urls.txt -name my_backup -dir ./downloads
```

### 3. Adjusting Worker Concurrency
Run 6 concurrent worker streams (default is 3):
```bash
downloader -file urls.txt -workers 6
```

### 4. Bandwidth Rate Limiting
Limit overall download speed across all workers to `5MB/s` (supports `B/s`, `KB/s`, `MB/s`, `GB/s`):
```bash
downloader -file urls.txt -limit 5MB/s
```

### 5. Single-File Download with SHA-256 Verification
Pass an expected SHA-256 hash on the CLI for verification:
```bash
downloader -file single_url.txt -sha256 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
```

### 6. Listing Saved Sessions
List all recorded sessions and their completion status:
```bash
downloader list
```

Example Output:
```
SESSION NAME         FILES      STATUS       LAST UPDATED        
-----------------------------------------------------------------
my_backup            12         8/12 Done    2026-09-27 10:15:30 
iso_downloads        4          4/4 Done     2026-09-26 18:42:11 
```

### 7. Inspecting Session Status
Inspect the status, individual file progress, and retry counts for a session:
```bash
downloader status my_backup
```

Example Output:
```
Session: my_backup
Output Directory: ./downloads
Updated: 2026-09-27 10:15:30

FILE                           STATUS       PROGRESS        RETRIES   
----------------------------------------------------------------------
ubuntu-22.04.4-desktop-amd64.iso COMPLETED    4.7 GiB / 4.7 GiB 0         
data_dump_2026.tar.gz          PAUSED       120.5 MiB / 1.5 GiB 0         
mini.iso                       COMPLETED    58.0 MiB / 58.0 MiB 0         
documentation.pdf              PAUSED       0 B / 15.2 MiB  1         
```

### 8. Resuming an Interrupted Session
Resume pending or paused files in an existing session with optional worker and speed limit overrides:
```bash
downloader resume my_backup -workers 4 -limit 10MB/s
```

The engine reloads the session state, inspects existing `.part` files on disk, issues HTTP Range requests with cache validator headers (`If-Range`), and finishes the downloads seamlessly without restarting from zero.

---

## Command-Line Reference

### Subcommands
- `downloader -file <path> [options]` : Initiates a download session.
- `downloader resume <session-name> [options]` : Resumes a saved session.
- `downloader list` : Lists all saved sessions.
- `downloader status <session-name>` : Displays detailed per-file progress for a session.

### Flags (`download`)
| Flag | Default | Description |
| :--- | :--- | :--- |
| `-file` | `""` | **Required.** Path to text file containing list of URLs. |
| `-name` | auto-generated | Session name used for state persistence. |
| `-dir` | `.` | Destination folder for completed downloads. |
| `-workers`| `3` | Number of concurrent worker goroutines. |
| `-limit` | `""` | Maximum global bandwidth (e.g. `5MB/s`, `800KB/s`). |
| `-sha256`| `""` | Expected SHA-256 hash for single-file integrity validation. |

### Flags (`resume`)
| Flag | Default | Description |
| :--- | :--- | :--- |
| `-workers`| `3` | Worker pool concurrency override. |
| `-limit` | `""` | Bandwidth rate limit override. |
