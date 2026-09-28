# Build & Compilation Guide

This document provides instructions for compiling, cross-compiling, testing, and distributing the Go Concurrent Resumable Downloader across different operating systems and package managers.

For CLI usage and examples, see the root [README.md](../README.md).  
For codebase design and component breakdown, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 1. Prerequisites

- **Go Compiler**: [Go 1.21 or later](https://go.dev/dl/) installed and in your system `PATH`.
- **Git**: Installed for version control and module management.
- **Terminal**:
  - Windows: PowerShell or Command Prompt.
  - Linux / macOS: Bash, Zsh, or any POSIX shell.

Verify your installed Go version:
```bash
go version
```

---

## 2. Quick Build

### Direct Go Build
To compile a stripped, standalone binary for your current operating system:

**On Windows (PowerShell / CMD):**
```powershell
go build -trimpath -ldflags="-s -w" -o bin/downloader.exe ./cmd/downloader
```

**On Linux / macOS (Bash):**
```bash
go build -trimpath -ldflags="-s -w" -o bin/downloader ./cmd/downloader
```

---

## 3. Automated Build Scripts

The repository includes pre-configured build scripts for all major operating systems and build environments.

### Using Make (Linux / macOS / Windows with GNU Make)
```bash
make build       # Builds binary for your current host OS/architecture in bin/
make windows     # Builds Windows binaries (amd64 and arm64) in bin/
make linux       # Builds Linux binaries (amd64 and arm64) in bin/
make all         # Builds both Linux and Windows binaries
make clean       # Cleans up all built binaries in bin/
make help        # Displays available targets and descriptions
```

### Windows PowerShell (`build.ps1`)
Runs automated dependency checks, tests, and builds binaries into `bin/`:
```powershell
.\build.ps1
```

### Windows Command Prompt (`build.bat`)
```cmd
build.bat
```

### Linux / macOS Shell Script (`build.sh`)
```bash
chmod +x ./build.sh
./build.sh
```

---

## 4. Cross-Compilation Matrix

The codebase is written in **100% pure Go** with zero CGO dependencies (`CGO_ENABLED=0`). This enables instant cross-compilation for any supported OS and architecture without installing cross-compilers or system headers.

### Windows
```bash
# Windows 64-bit (x86_64)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/downloader-windows-amd64.exe ./cmd/downloader

# Windows ARM64 (Snapdragon / Surface Pro X)
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/downloader-windows-arm64.exe ./cmd/downloader
```

### Linux
```bash
# Linux 64-bit (x86_64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/downloader-linux-amd64 ./cmd/downloader

# Linux ARM64 (Raspberry Pi 4/5, AWS Graviton)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/downloader-linux-arm64 ./cmd/downloader
```

### macOS (Darwin)
```bash
# macOS Apple Silicon (M1/M2/M3/M4)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/downloader-darwin-arm64 ./cmd/downloader

# macOS Intel (x86_64)
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/downloader-darwin-amd64 ./cmd/downloader
```

---

## 5. Important Compilation Details & Flags

| Flag | Purpose |
| :--- | :--- |
| `CGO_ENABLED=0` | Disables CGO. Produces fully static, self-contained binaries that run on any Linux distribution without glibc version mismatches or musl incompatibilities (e.g. Alpine Linux). |
| `-trimpath` | Strips absolute file system paths from compiled stack traces. Enhances build reproducibility and avoids leaking developer directory paths. |
| `-ldflags="-s -w"` | Strips symbol tables (`-s`) and DWARF debugging information (`-w`). Reduces final binary size by approximately 30–40% without impacting runtime performance. |

---

## 6. Running Tests & Race Detection

### Standard Unit Tests
Run all unit tests across all packages:
```bash
go test -v ./...
```

### Race Detector
Verify thread safety, mutex synchronizations, and absence of data races:
```bash
go test -race ./...
```

### Specific Package Testing
Test the authentication vault and strategies:
```bash
go test -v ./internal/auth
```

Test the grab transfer and resumption engine:
```bash
go test -v ./internal/transfer
```

---

## 7. Dependency Management

The project uses standard Go modules (`go.mod` and `go.sum`).

- **Verify dependencies**:
  ```bash
  go mod verify
  ```
- **Download and cache dependencies**:
  ```bash
  go mod download
  ```
- **Tidy and clean unused modules**:
  ```bash
  go mod tidy
  ```

---

## 8. Distribution Channels & Package Managers

For command-line tools, developers prefer installing via their native terminal package manager. Below is the distribution strategy and automated release pipeline.

### 8.1. Distribution Matrix

| Platform | Channel | Install Command | Submission & Maintenance |
| :--- | :--- | :--- | :--- |
| **Windows** | **WinGet** *(Native)* | `winget install <id>.downloader` | Submit manifest to [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs) using [`wingetcreate`](https://github.com/microsoft/wingetcreate). |
| **Windows** | **Scoop** *(Dev favorite)* | `scoop install downloader` | Automated via GoReleaser into a `scoop-bucket` GitHub repo. |
| **macOS** | **Homebrew** | `brew install yourorg/tap/downloader` | Automated via GoReleaser into a `homebrew-tap` GitHub repo. |
| **Linux** | **Debian / Ubuntu (`.deb`)** | `sudo dpkg -i downloader_*.deb` | Packaged automatically as release artifacts by GoReleaser (`nfpm`). |
| **Linux** | **Fedora / RHEL (`.rpm`)** | `sudo rpm -i downloader_*.rpm` | Packaged automatically as release artifacts by GoReleaser (`nfpm`). |
| **Linux** | **Arch Linux (AUR)** | `yay -S downloader-bin` | Managed via `PKGBUILD` pointing to the GitHub release archive. |
| **Universal**| **Go Toolchain** | `go install github.com/yourorg/downloader/cmd/downloader@latest` | Native to all Go developers worldwide. |
| **Universal**| **GitHub Releases** | Download standalone zip / tar.gz | Multi-OS pre-compiled binaries with SHA-256 `checksums.txt`. |

> [!NOTE]
> **Why avoid the Microsoft Store for this tool:**  
> The Microsoft Store targets GUI desktop apps and requires MSIX containers, developer accounts, annual fees, and app-sandbox restrictions that interfere with terminal tooling and custom download paths. **WinGet** and **Scoop** are the designated, zero-friction standards for Windows CLI tools.

---

### 8.2. Automated Release Pipeline with GoReleaser

The project includes a root [`.goreleaser.yaml`](../.goreleaser.yaml) and GitHub Actions workflow [`.github/workflows/release.yml`](../.github/workflows/release.yml).

Whenever a semantic version tag is pushed:
```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

The GitHub Actions workflow triggers GoReleaser to automatically:
1. Compile stripped, reproducible binaries across Windows, Linux, and macOS (amd64 and arm64).
2. Bundle `.zip` (Windows) and `.tar.gz` (Linux/macOS) archives with documentation.
3. Generate native `.deb` and `.rpm` packages for Linux server distributions.
4. Calculate SHA-256 cryptographic hashes and produce `checksums.txt`.
5. Update your **Homebrew Tap** formula and **Scoop Bucket** manifest.
6. Publish a formal GitHub Release with auto-generated changelogs.

---

### 8.3. Windows WinGet Submission (`wingetcreate`)

To make the tool installable via `winget install downloader`:

1. Download the Microsoft [`wingetcreate`](https://github.com/microsoft/wingetcreate/releases) CLI tool.
2. Generate and submit the manifest using your GitHub release URL:
   ```powershell
   wingetcreate new https://github.com/yourorg/downloader/releases/download/v1.0.0/downloader_1.0.0_windows_amd64.zip
   ```
3. Follow the interactive prompts (package identifier, publisher name, license).
4. `wingetcreate` automatically tests the manifest and submits a Pull Request to [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs). Once merged, users can install with a single command:
   ```cmd
   winget install downloader
   ```

---

### 8.4. Local Release Preview (Dry-Run)

You can preview the GoReleaser build locally without publishing:

```bash
# Check configuration syntax
goreleaser check

# Build test archives and packages in dist/ without uploading
goreleaser release --snapshot --clean
```
