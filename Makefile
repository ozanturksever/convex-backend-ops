# convex-backend-ops Makefile

# Variables
BINARY_NAME := convex-backend-ops
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/ozanturksever/convex-backend-ops/cmd.Version=$(VERSION) -X github.com/ozanturksever/convex-backend-ops/cmd.GitCommit=$(GIT_COMMIT) -X github.com/ozanturksever/convex-backend-ops/cmd.BuildTime=$(BUILD_TIME)"

# Directories
DIST_DIR := dist
BUNDLE_DIR := testdata/sample-bundle

# Platforms for release
PLATFORMS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

.PHONY: all build clean test test-short test-integration lint fmt help
.PHONY: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64
.PHONY: release bundle

# Default target
all: build

## Build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_NAME) .

# Build for Linux AMD64
build-linux-amd64:
	@mkdir -p $(DIST_DIR)
	@echo "Building $(BINARY_NAME) for linux-amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .

# Build for Linux ARM64
build-linux-arm64:
	@mkdir -p $(DIST_DIR)
	@echo "Building $(BINARY_NAME) for linux-arm64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .

# Build for macOS AMD64
build-darwin-amd64:
	@mkdir -p $(DIST_DIR)
	@echo "Building $(BINARY_NAME) for darwin-amd64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .

# Build for macOS ARM64
build-darwin-arm64:
	@mkdir -p $(DIST_DIR)
	@echo "Building $(BINARY_NAME) for darwin-arm64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .

# Build for all platforms
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64
	@echo "All builds complete!"

## Testing

# Run all tests (requires Docker)
test: build-for-test
	@echo "Running all tests..."
	go test -v -timeout 600s ./...

# Run only unit tests (no Docker required)
test-short:
	@echo "Running unit tests..."
	go test -short -v ./...

# Run only integration tests (requires Docker)
test-integration: build-for-test
	@echo "Running integration tests..."
	go test -v -timeout 600s -run Integration ./...

# Build binary for integration tests (matches host architecture for Docker)
build-for-test:
	@echo "Building $(BINARY_NAME) for integration tests..."
	@if [ "$$(uname -m)" = "arm64" ]; then \
		CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME) .; \
	else \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME) .; \
	fi

## Code Quality

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Verify dependencies
verify:
	@echo "Verifying dependencies..."
	go mod verify
	go mod tidy

## Release

# Create release artifacts
release: clean build-all
	@echo "Creating release artifacts..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		echo "Creating archive for $$platform..."; \
		cd $(DIST_DIR) && tar -czvf $(BINARY_NAME)-$$platform.tar.gz $(BINARY_NAME)-$$platform && cd ..; \
	done
	@echo "Release artifacts created in $(DIST_DIR)/"
	@ls -la $(DIST_DIR)/

# Create checksums for release artifacts
checksums:
	@echo "Creating checksums..."
	@cd $(DIST_DIR) && shasum -a 256 *.tar.gz > checksums.txt
	@cat $(DIST_DIR)/checksums.txt

## Bundle (for testing)

# Create test bundle using convex-bundler (requires convex-bundler to be installed)
bundle:
	@echo "Creating test bundle..."
	@if [ ! -d "../2025-12-29-convex-app-bundler" ]; then \
		echo "Error: convex-bundler not found at ../2025-12-29-convex-app-bundler"; \
		exit 1; \
	fi
	cd ../2025-12-29-convex-app-bundler && \
		./convex-bundler --app ./testdata/sample-app \
			--output ../2025-12-30-convex-app-installer/$(BUNDLE_DIR) \
			--backend-binary ./bin/convex-local-backend \
			--name "Test Backend" \
			--platform linux-arm64
	@echo "Bundle created at $(BUNDLE_DIR)/"

## Cleanup

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -rf $(DIST_DIR)

# Clean everything including test data
clean-all: clean
	rm -f $(BUNDLE_DIR)/backend
	rm -f $(BUNDLE_DIR)/convex.db

## Install

# Install to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) .

## Help

help:
	@echo "convex-backend-ops Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build targets:"
	@echo "  build              Build for current platform"
	@echo "  build-linux-amd64  Build for Linux AMD64"
	@echo "  build-linux-arm64  Build for Linux ARM64"
	@echo "  build-darwin-amd64 Build for macOS AMD64"
	@echo "  build-darwin-arm64 Build for macOS ARM64"
	@echo "  build-all          Build for all platforms"
	@echo ""
	@echo "Test targets:"
	@echo "  test               Run all tests (requires Docker)"
	@echo "  test-short         Run unit tests only (no Docker)"
	@echo "  test-integration   Run integration tests only (requires Docker)"
	@echo ""
	@echo "Code quality:"
	@echo "  lint               Run golangci-lint"
	@echo "  fmt                Format code"
	@echo "  verify             Verify dependencies"
	@echo ""
	@echo "Release targets:"
	@echo "  release            Build and package for all platforms"
	@echo "  checksums          Generate checksums for release artifacts"
	@echo ""
	@echo "Other targets:"
	@echo "  bundle             Create test bundle using convex-bundler"
	@echo "  install            Install to GOPATH/bin"
	@echo "  clean              Clean build artifacts"
	@echo "  clean-all          Clean everything including test data"
	@echo "  help               Show this help"
