# cwd — self-hosted Critical Weather Day Status
# Default target prints help.

GO          ?= go
BIN_DIR     ?= bin
BIN          := $(BIN_DIR)/cwd
PKG          := github.com/acaudill/cwd

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE        := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -X $(PKG)/internal/version.Version=$(VERSION) \
               -X $(PKG)/internal/version.Commit=$(COMMIT) \
               -X $(PKG)/internal/version.Date=$(DATE)

.PHONY: help build test lint run clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the cwd binary (assumes web/dist already populated)
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/cwd

test: ## Run backend unit tests
	$(GO) test ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

run: ## Run cwd serve with current code (no rebuild of frontend)
	$(GO) run ./cmd/cwd serve

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
