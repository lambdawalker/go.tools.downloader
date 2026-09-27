#!/usr/bin/env bash
# ==============================================================================
# Build script for Downloader (Linux / Unix / macOS / Git Bash)
# ==============================================================================

set -euo pipefail

APP_NAME="downloader"
PACKAGE="./cmd/downloader"
OUTPUT_DIR="bin"
LDFLAGS="-s -w"

# Print usage information
usage() {
    echo "Usage: $0 [TARGET]"
    echo ""
    echo "Targets:"
    echo "  all       Build binaries for both Linux and Windows (amd64 and arm64)"
    echo "  linux     Build Linux binaries (amd64 and arm64)"
    echo "  windows   Build Windows binaries (amd64 and arm64)"
    echo "  current   Build binary for the current OS/architecture (default)"
    echo "  clean     Remove built binaries in '${OUTPUT_DIR}/'"
    echo "  help      Show this help message"
    echo ""
}

clean() {
    echo "Cleaning output directory '${OUTPUT_DIR}'..."
    rm -rf "${OUTPUT_DIR}"
    echo "Clean complete."
}

build_binary() {
    local target_os="$1"
    local target_arch="$2"
    local ext=""
    if [ "$target_os" = "windows" ]; then
        ext=".exe"
    fi

    local output_file="${OUTPUT_DIR}/${APP_NAME}-${target_os}-${target_arch}${ext}"

    echo "Building ${APP_NAME} for ${target_os}/${target_arch} -> ${output_file}..."
    CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" \
        go build -trimpath -ldflags="${LDFLAGS}" -o "${output_file}" "${PACKAGE}"
}

build_linux() {
    mkdir -p "${OUTPUT_DIR}"
    build_binary "linux" "amd64"
    build_binary "linux" "arm64"
    # Create default binary for linux
    cp -f "${OUTPUT_DIR}/${APP_NAME}-linux-amd64" "${OUTPUT_DIR}/${APP_NAME}" || true
}

build_windows() {
    mkdir -p "${OUTPUT_DIR}"
    build_binary "windows" "amd64"
    build_binary "windows" "arm64"
    # Create default binary for windows
    cp -f "${OUTPUT_DIR}/${APP_NAME}-windows-amd64.exe" "${OUTPUT_DIR}/${APP_NAME}.exe" || true
}

build_current() {
    mkdir -p "${OUTPUT_DIR}"
    local host_os
    local host_arch
    host_os="$(go env GOOS)"
    host_arch="$(go env GOARCH)"

    local ext=""
    if [ "$host_os" = "windows" ]; then
        ext=".exe"
    fi

    local output_file="${OUTPUT_DIR}/${APP_NAME}${ext}"
    echo "Building ${APP_NAME} for host platform (${host_os}/${host_arch}) -> ${output_file}..."
    CGO_ENABLED=0 go build -trimpath -ldflags="${LDFLAGS}" -o "${output_file}" "${PACKAGE}"
}

# Parse argument
TARGET="${1:-current}"

case "${TARGET}" in
    help|-h|--help)
        usage
        exit 0
        ;;
    clean)
        clean
        exit 0
        ;;
esac

# Ensure Go is available
if ! command -v go >/dev/null 2>&1; then
    echo "[ERROR] 'go' command not found. Please install Go (https://golang.org/dl/) or add it to PATH." >&2
    exit 1
fi

case "${TARGET}" in
    all)
        echo "=== Building all targets (Linux & Windows) ==="
        build_linux
        build_windows
        echo "=== Build finished successfully! Outputs in '${OUTPUT_DIR}/' ==="
        ;;
    linux|lunix)
        echo "=== Building Linux targets ==="
        build_linux
        echo "=== Build finished successfully! Outputs in '${OUTPUT_DIR}/' ==="
        ;;
    windows|win)
        echo "=== Building Windows targets ==="
        build_windows
        echo "=== Build finished successfully! Outputs in '${OUTPUT_DIR}/' ==="
        ;;
    current)
        echo "=== Building current host target ==="
        build_current
        echo "=== Build finished successfully! Output in '${OUTPUT_DIR}/' ==="
        ;;
    *)
        echo "[ERROR] Unknown target: ${TARGET}" >&2
        usage
        exit 1
        ;;
esac
