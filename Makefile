# Project-specific variables (customize for each project)
BINARY_NAME=curate-preservation-core
MODULE_NAME=github.com/penwern/curate-preservation-core
VERSION?=dev
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X $(MODULE_NAME)/cmd.Version=$(VERSION) -X $(MODULE_NAME)/cmd.GitCommit=$(GIT_COMMIT) -X $(MODULE_NAME)/cmd.BuildDate=$(BUILD_DATE)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOINSTALL=$(GOCMD) install
GOMOD=$(GOCMD) mod

# Buf parameters
BUFCMD=buf
PROTO_DIR=common/proto/a3m

# Find all Go files excluding proto-generated files
GO_FILES := $(shell find . -name "*.go" -not -name "*.pb.go" -not -path "./vendor/*")

# Default target
.PHONY: all
all: build

# Build targets
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

.PHONY: build-all
build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .

.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOINSTALL) $(LDFLAGS) .

# Dependency management
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

.PHONY: mod-tidy
mod-tidy:
	$(GOMOD) tidy
	$(GOMOD) verify

# Testing targets
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

.PHONY: test-coverage
test-coverage:
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Protocol Buffer targets
.PHONY: buf-generate
buf-generate:
	@echo "Running buf generate..."
	cd $(PROTO_DIR) && $(BUFCMD) generate

# Formatting targets
.PHONY: format
format:
	@echo "Running goimports..."
	@goimports -w $(GO_FILES)
	@echo "Running gci..."
	@gci write --skip-generated -s standard -s default -s "prefix($$(go list -m))" .
	@echo "Running gofumpt..."
	@gofumpt -w $(GO_FILES)
	@echo "Formatting complete!"

.PHONY: format-check
format-check:
	@echo "Checking if files are formatted..."
	@if [ -n "$$(goimports -d $(GO_FILES))" ]; then \
		echo "Files are not formatted. Run 'make format' to fix."; \
		goimports -d $(GO_FILES); \
		exit 1; \
	fi
	@echo "All files are properly formatted!"

# Linting targets
.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./... --config .golangci.yml

.PHONY: lint-fix
lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	golangci-lint run ./... --config .golangci.yml --fix

.PHONY: lint-verbose
lint-verbose:
	@echo "Running golangci-lint with verbose output..."
	golangci-lint run ./... --config .golangci.yml --verbose

# Combined targets
.PHONY: check
check: format-check lint test
	@echo "All checks passed!"

.PHONY: fix
fix: format lint-fix
	@echo "Auto-fixing complete!"

.PHONY: pre-commit
pre-commit: fix test
	@echo "Pre-commit checks completed!"

# Runtime targets
.PHONY: run
run:
	@echo "Running $(BINARY_NAME)..."
	$(GOCMD) run $(LDFLAGS) . --serve

# Docker targets
.PHONY: setup-volumes
setup-volumes:
	@echo "Creating Docker volumes directories..."
	sudo mkdir -p /tmp/preservation
	sudo mkdir -p /tmp/preservation/a3m_completed
	sudo mkdir -p /tmp/preservation/a3m_dips
	sudo chown -R 1000:1000 /tmp/preservation
	sudo chmod -R 755 /tmp/preservation
	@echo "Directories created with proper permissions:"
	@echo "- /tmp/preservation"
	@echo "- /tmp/preservation/a3m_completed"
	@echo "- /tmp/preservation/a3m_dips"

.PHONY: docker-up
docker-up: setup-volumes
	@echo "Starting Docker Compose services..."
	docker compose up -d

.PHONY: docker-down
docker-down:
	@echo "Stopping Docker Compose services..."
	docker compose down

.PHONY: docker-logs
docker-logs:
	@echo "Showing Docker Compose logs..."
	docker compose logs -f

# Clean targets
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -f coverage.out coverage.html

.PHONY: clean-cache
clean-cache:
	$(GOCLEAN) -cache
	$(GOCLEAN) -modcache

# Development tools installation
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	$(GOINSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOINSTALL) golang.org/x/tools/cmd/goimports@latest
	$(GOINSTALL) github.com/daixiang0/gci@latest
	$(GOINSTALL) mvdan.cc/gofumpt@latest
	@echo "Installing buf..."
	@curl -sSL "https://github.com/bufbuild/buf/releases/latest/download/buf-$$(uname -s)-$$(uname -m)" -o "$${HOME}/.local/bin/buf" && chmod +x "$${HOME}/.local/bin/buf"
	@echo "Development tools installed!"

# CI/CD targets
.PHONY: ci
ci: mod-tidy check build
	@echo "CI pipeline completed successfully!"

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build:"
	@echo "  build         - Build the binary"
	@echo "  build-all     - Build for multiple platforms"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo ""
	@echo "Dependencies:"
	@echo "  deps          - Install and tidy dependencies"
	@echo "  mod-tidy      - Tidy and verify go modules"
	@echo ""
	@echo "Testing:"
	@echo "  test          - Run all tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  format        - Format all Go files"
	@echo "  format-check  - Check if files are formatted"
	@echo "  lint          - Run golangci-lint"
	@echo "  lint-fix      - Run linter with auto-fix"
	@echo "  lint-verbose  - Run linter with verbose output"
	@echo ""
	@echo "Combined:"
	@echo "  check         - Run format-check, lint, and test"
	@echo "  fix           - Run format and lint-fix"
	@echo "  pre-commit    - Run fix and test (good for git hooks)"
	@echo "  ci            - Full CI pipeline"
	@echo ""
	@echo "Runtime:"
	@echo "  run           - Run the application"
	@echo ""
	@echo "Docker:"
	@echo "  setup-volumes - Create Docker volume directories with proper permissions"
	@echo "  docker-up     - Start Docker Compose services (includes setup-volumes)"
	@echo "  docker-down   - Stop Docker Compose services"
	@echo "  docker-logs   - Show Docker Compose logs"
	@echo ""
	@echo "Clean:"
	@echo "  clean         - Clean build artifacts"
	@echo "  clean-cache   - Clean Go caches"
	@echo "  install-tools - Install development tools"
	@echo "  help          - Show this help message"