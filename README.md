# Go Concurrent Resumable Downloader

> [!NOTE]
> **Documentation Guide**  
> This document is the primary **User Guide** focused on how to configure and use the `downloader` tool.
> - For internal code design, subsystem mechanics, and concurrency architecture, see [ARCHITECTURE.md](docs/ARCHITECTURE.md).
> - For compilation instructions, build scripts, and cross-compilation options, see [BUILD.md](docs/BUILD.md).

A high-performance command-line download manager built in Go. It accelerates batch downloads through multi-worker streaming, automatic byte-range resumption, pre-flight metadata inspection, stall and slow-stream demotion, bandwidth throttling, master-password-encrypted credential vaults, and an interactive real-time ANSI terminal dashboard.

---

## Quick Navigation

- [Quick Start](#quick-start)
- [Input File Format (`urls.txt`)](#input-file-format-urlstxt)
- [Encrypted Vault & Authentication](#encrypted-vault--authentication)
- [CLI Commands & Subcommands](#cli-commands--subcommands)
- [Practical Usage Examples](#practical-usage-examples)
- [Flags & Options Reference](#flags--options-reference)
- [Documentation Index](#documentation-index)

---

## Quick Start

### 1. Compile the Binary
```bash
go build -trimpath -ldflags="-s -w" -o bin/downloader.exe ./cmd/downloader
```
*(For Make, PowerShell, or cross-compilation instructions, see [BUILD.md](docs/BUILD.md)).*

### 2. Create a URL List (`urls.txt`)
```text
https://releases.ubuntu.com/22.04.4/ubuntu-22.04.4-desktop-amd64.iso
https://deb.debian.org/debian/dists/bookworm/main/installer-amd64/current/images/netboot/mini.iso
```

### 3. Run the Downloader
```bash
./bin/downloader -file urls.txt
```

---

## Input File Format (`urls.txt`)

The input file is a plain-text file listing target URLs (one per line). Empty lines and comments (`#`) are ignored.

You can augment each line with optional directive tags in any order:
- `auth=<profile>`: Assigns a named credential profile from your encrypted vault.
- `sha256=<hash>`: Enforces post-download SHA-256 integrity verification before promoting the file.

### Example `urls.txt`
```text
# Public downloads (anonymous access)
https://releases.ubuntu.com/22.04.4/ubuntu-22.04.4-desktop-amd64.iso

# Private GitHub release using the "github-prod" encrypted vault profile
https://api.github.com/repos/myorg/project/tarball auth=github-prod

# Internal repository package with both authentication and SHA-256 verification
https://nexus.corp.internal/repository/raw/asset.tar.gz auth=corp-nexus sha256=2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824

# Backwards compatible syntax (bare SHA-256 hash separated by space)
https://example.com/data/archive.tar.gz 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
```

> [!TIP]
> Storing profile names instead of raw tokens in `urls.txt` allows you to safely commit download lists to Git repositories without leaking secrets.

---

## Encrypted Vault & Authentication

The tool includes a built-in credential vault located at `~/.downloader/auth/vault.enc`. All secrets (tokens, passwords, API keys) are protected with **AES-256-GCM** authenticated encryption and **PBKDF2-HMAC-SHA256** (100,000 iterations).

Tokens exist **in RAM only** while the program runs and are never saved in plaintext on disk or exposed in environment variables.

### 1. Initialize the Vault
Create a new encrypted vault and set your master password:
```bash
downloader auth vault init
```
```text
[Vault] Enter Master Password for new vault: **********
Confirm Master Password: **********
[Success] Encrypted credential vault initialized successfully at: ~/.downloader/auth/vault.enc
```

### 2. Configure Named Auth Profiles
Add credential profiles to your vault. The CLI will prompt you interactively:

```bash
# Add a Bearer Token profile (e.g. GitHub, GitLab, HuggingFace)
downloader auth profile set github-prod

# Add HTTP Basic Auth credentials (e.g. Artifactory, Nexus)
downloader auth profile set corp-nexus

# Add Custom HTTP Headers (e.g. X-API-Key, Private-Token)
downloader auth profile set custom-gateway
```

Interactive setup example:
```text
Configure credentials for profile "github-prod":
1. Bearer Token
2. Basic Auth (username & password)
3. Custom HTTP Headers (API Key, Personal Access Token)
Select method [1-3] (default 1): 1
Enter Bearer Token: ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
[Success] Profile "github-prod" encrypted and saved into vault.
```

### 3. Map a Domain to a Default Profile (Optional)
If a URL in `urls.txt` does not specify an `auth=` tag, you can bind an entire domain to an encrypted profile:
```bash
downloader auth domain set api.github.com github-prod
```

### 4. List Vault Profiles
```bash
downloader auth profile list
```
Output:
```text
PROFILE NAME              METHOD          DETAILS                       
----------------------------------------------------------------------
github-prod               bearer          Token: ghp...xxx              
corp-nexus                basic           User: deploy-bot              
custom-gateway            header          1 headers configured          
```

### 5. Change Master Password
Re-encrypt the vault with a new master password and a fresh cryptographic salt:
```bash
downloader auth vault passwd
```

---

## CLI Commands & Subcommands

| Command | Description |
| :--- | :--- |
| `downloader -file <path> [options]` | Initiates a batch download session. |
| `downloader resume <session-name> [options]` | Resumes incomplete downloads from a previous session. |
| `downloader list` | Lists all recorded sessions, file counts, and statuses. |
| `downloader status <session-name>` | Displays detailed progress, file sizes, and retry counts for a session. |
| `downloader auth list` | Displays an overview of vault status and configured profiles. |
| `downloader auth vault init` | Initializes a new encrypted credential vault. |
| `downloader auth vault passwd` | Updates the master password and re-encrypts the vault. |
| `downloader auth profile set <name>` | Interactively adds or updates an encrypted credential profile. |
| `downloader auth profile list` | Unlocks and lists all configured profiles. |
| `downloader auth profile remove <name>`| Deletes a credential profile from the vault. |
| `downloader auth domain set <domain> <prof>` | Maps a domain name to a default vault profile. |

---

## Practical Usage Examples

### 1. Basic Batch Download
Download all files listed in `urls.txt` into the current directory using 3 concurrent workers:
```bash
downloader -file urls.txt
```

### 2. Custom Output Folder and Session Name
Download files into `./downloads` under a custom session name (`backup_2026`):
```bash
downloader -file urls.txt -dir ./downloads -name backup_2026
```

### 3. Adjusting Worker Concurrency
Download with 8 concurrent worker streams:
```bash
downloader -file urls.txt -workers 8
```

### 4. Bandwidth Rate Limiting
Throttle overall download speed to `10MB/s` (supports `B/s`, `KB/s`, `MB/s`, `GB/s`):
```bash
downloader -file urls.txt -limit 10MB/s
```

### 5. Automated / Headless Runs (CI/CD Pipelines)
When running unattended in CI/CD runners or cron jobs, supply the vault master password non-interactively:

**Using a protected password file:**
```bash
downloader -file urls.txt -password-file /run/secrets/vault.key
```

**Using standard input (pipe):**
```bash
echo "$VAULT_PASSWORD" | downloader -file urls.txt -password-stdin
```

### 6. Inspecting Saved Sessions
List all recorded sessions:
```bash
downloader list
```
Output:
```text
SESSION NAME         FILES      STATUS       LAST UPDATED        
-----------------------------------------------------------------
backup_2026          12         8/12 Done    2026-09-27 10:15:30 
iso_downloads        4          4/4 Done     2026-09-26 18:42:11 
```

View detailed progress of an individual session:
```bash
downloader status backup_2026
```
Output:
```text
Session: backup_2026
Output Directory: ./downloads
Updated: 2026-09-27 10:15:30

FILE                           STATUS       PROGRESS        RETRIES   
----------------------------------------------------------------------
ubuntu-22.04.4-desktop.iso     COMPLETED    4.7 GiB / 4.7 GiB 0         
data_dump.tar.gz               PAUSED       120.5 MiB / 1.5 GiB 0         
mini.iso                       COMPLETED    58.0 MiB / 58.0 MiB 0         
```

### 7. Resuming an Interrupted Session
Resume pending or paused downloads from a saved session with updated concurrency and speed limits:
```bash
downloader resume backup_2026 -workers 6 -limit 20MB/s
```

The engine reloads existing `.part` files on disk, verifies cache validators (`If-Range`), and resumes streaming without restarting from zero.

---

## Flags & Options Reference

### `download` Flags
| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-file` | string | `""` | **Required.** Path to text file containing URLs. |
| `-name` | string | auto | Session name for state persistence. |
| `-dir` | string | `.` | Target destination directory for completed files. |
| `-workers` | int | `3` | Number of concurrent worker goroutines. |
| `-limit` | string | `""` | Maximum global bandwidth limit (e.g. `5MB/s`, `500KB/s`). |
| `-sha256` | string | `""` | Expected SHA-256 hash for single-file integrity check. |
| `-password-file` | string | `""` | Path to file containing the vault master password. |
| `-password-stdin`| bool | `false`| Read the vault master password from standard input. |

### `resume` Flags
| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-workers` | int | `3` | Worker pool concurrency override. |
| `-limit` | string | `""` | Bandwidth rate limit override. |
| `-password-file` | string | `""` | Path to file containing the vault master password. |
| `-password-stdin`| bool | `false`| Read the vault master password from standard input. |

---

## Documentation Index

- [Architecture & Internal Design](docs/ARCHITECTURE.md)
  - Subsystem breakdown and Mermaid data flow diagrams
  - Shortest-Job-First (SJF) queue scheduling
  - Adaptive stall and slow-stream demotion watchdog
  - Domain-scoped security transports and anti-leak redirect stripping
  - Thread safety and concurrency model
- [Build & Compilation Guide](docs/BUILD.md)
  - Prerequisites and compiler setup
  - Makefile and shell build scripts (`build.ps1`, `build.bat`, `build.sh`)
  - Cross-compilation matrix (Windows, Linux, macOS for amd64 and arm64)
  - Static binaries, CGO-free configuration, and size optimizations
  - Testing suite and race detector execution
- Package Documentation:
  - [`cmd/downloader`](cmd/downloader/README.md) — CLI entry point and flag dispatcher
  - [`internal/auth`](internal/auth/README.md) — Encrypted vault, auth strategies, and transport
  - [`internal/transfer`](internal/transfer/README.md) — Grab streaming and resumption engine
  - [`internal/dashboard`](internal/dashboard/README.md) — ANSI terminal UI renderer
  - [`internal/engine`](internal/engine/README.md) — Session and worker pool orchestrator
  - [`internal/prober`](internal/prober/README.md) — Pre-flight metadata and range inspector
  - [`internal/queue`](internal/queue/README.md) — SJF priority queue
  - [`internal/watchdog`](internal/watchdog/README.md) — Stall and slow-stream detector
  - [`internal/limiter`](internal/limiter/README.md) — Token-bucket rate limiter
  - [`internal/store`](internal/store/README.md) — Session persistence store
  - [`internal/checksum`](internal/checksum/README.md) — SHA-256 verification
  - [`internal/model`](internal/model/README.md) — Domain types and data models
