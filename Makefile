# ==============================================================================
# Makefile for Downloader (Linux / Unix / macOS / Windows with Make)
# ==============================================================================

APP_NAME    := downloader
PACKAGE     := ./cmd/downloader
OUTPUT_DIR  := bin
LDFLAGS     := -s -w

.PHONY: all build linux windows clean help

all: linux windows ## Build binaries for both Linux and Windows

build: ## Build binary for the current host OS/architecture
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(APP_NAME) $(PACKAGE)

linux: ## Build Linux binaries (amd64 and arm64)
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(APP_NAME)-linux-amd64 $(PACKAGE)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(APP_NAME)-linux-arm64 $(PACKAGE)
	@cp -f $(OUTPUT_DIR)/$(APP_NAME)-linux-amd64 $(OUTPUT_DIR)/$(APP_NAME) 2>/dev/null || true

windows: ## Build Windows binaries (amd64 and arm64)
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(APP_NAME)-windows-amd64.exe $(PACKAGE)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(APP_NAME)-windows-arm64.exe $(PACKAGE)
	@cp -f $(OUTPUT_DIR)/$(APP_NAME)-windows-amd64.exe $(OUTPUT_DIR)/$(APP_NAME).exe 2>/dev/null || true

clean: ## Clean build artifacts in bin/
	rm -rf $(OUTPUT_DIR)

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
