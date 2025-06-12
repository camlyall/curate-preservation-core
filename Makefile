# Go parameters
BINARY_NAME=preservation-core
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

.PHONY: build
build:
	$(GOBUILD) -o $(BINARY_NAME) ./...

.PHONY: install
install:
	$(GOINSTALL) ./...

.PHONY: test
test:
	$(GOTEST) -v ./...

.PHONY: test-coverage
test-coverage:
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

.PHONY: mod-tidy
mod-tidy:
	$(GOMOD) tidy
	$(GOMOD) verify

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

# Clean targets
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f bin/*
	rm -f coverage.out coverage.html

.PHONY: clean-cache
clean-cache:
	$(GOCLEAN) -cache
	$(GOCLEAN) -modcache

# Install development tools
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	$(GOINSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOINSTALL) golang.org/x/tools/cmd/goimports@latest
	$(GOINSTALL) github.com/daixiang0/gci@latest
	$(GOINSTALL) mvdan.cc/gofumpt@latest
	@echo "Installing buf..."
	@curl -sSL "https://github.com/bufbuild/buf/releases/latest/download/buf-$$(uname -s)-$$(uname -m)" -o "$${HOME}/.local/bin/buf" && chmod +x "$${HOME}/.local/bin/buf"
	@echo "Tools installed!"

# CI/CD targets
.PHONY: ci
ci: mod-tidy check build
	@echo "CI pipeline completed successfully!"

.PHONY: pre-commit
pre-commit: fix test
	@echo "Pre-commit checks completed!"

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build all packages"
	@echo "  install       - Install all packages"
	@echo "  test          - Run all tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  mod-tidy      - Tidy and verify go modules"
	@echo ""
	@echo "Protocol Buffers:"
	@echo "  buf-generate  - Generate code from proto files"
	@echo ""
	@echo "Formatting:"
	@echo "  format        - Format all Go files"
	@echo "  format-check  - Check if files are formatted"
	@echo ""
	@echo "Linting:"
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
	@echo "Maintenance:"
	@echo "  clean         - Clean build artifacts"
	@echo "  clean-cache   - Clean Go caches"
	@echo "  install-tools - Install development tools"
	@echo "  help          - Show this help message"