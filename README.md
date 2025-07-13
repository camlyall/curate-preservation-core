# Curate Preservation Core

[![Go Report Card](https://goreportcard.com/badge/github.com/penwern/curate-preservation-core)](https://goreportcard.com/report/github.com/penwern/curate-preservation-core)
[![Go Version](https://img.shields.io/badge/go-1.24.1+-blue.svg)](https://golang.org/dl/)

A digital preservation service that integrates with Penwern A3M archival processing and Pydio Cells file management for comprehensive digital archival workflows. This service orchestrates the complete preservation pipeline from file selection to archival storage. AKA CA4M.

## 🌟 Key Features

- **Automated Preservation Workflows** - Seamless integration with A3M for archival processing
- **Metadata Management** - Tracks preservation status through Pydio Cells metadata
- **RESTful API** - HTTP endpoints for preservation operations
- **Command Line Interface** - Direct CLI access for administrative tasks
- **Docker Support** - Containerized deployment with development environment
- **PREMIS Integration** - Standards-compliant preservation metadata
- **Flexible Configuration** - Support for command-line flags, environment variables, and configuration files
- **Multi-format Processing** - Handles diverse file types and package formats
- **Status Tracking** - Real-time preservation workflow monitoring
- **AtoM Integration** - Optional archival description linking

## 🏗️ Architecture

The service acts as a bridge between:

- **Pydio Cells** - File management and metadata storage
- **A3M/A4M** - Archival processing engine
- **AtoM** - Archival description and access (optional)

```
[Pydio Cells] ↔ [Preservation Core] ↔ [A3M Processing] → [Pydio Cells Archive Storage]
                        ↓
                   [AtoM Integration]
```

## 🔧 Dependencies

### Required
- **Penwern A3M (A4M)** - Shared file system required for processing
- **Cells Enterprise Client Binary (CEC)** - For Pydio Cells integration
- **Pydio Cells** - With configured metadata namespaces
- **libxml2-utils** - For XML Schema validation (PREMIS validation)

### Optional
- **AtoM** - For archival description integration
- **Docker** - For containerized deployment

### Metadata Namespaces

The following Pydio Cells metadata namespaces must be configured:

- `usermeta-preservation-status` (required) - Tracks preservation workflow status
- `usermeta-dip-status` (optional) - Dissemination Information Package status
- `usermeta-atom-slug` (optional) - AtoM archival description linking

> **Important**: Metadata namespaces must be editable by users. Admin users cannot edit personal file tags.

> **Migration Notes**: 
> - `usermeta-preservation-status` replaces deprecated `usermeta-a3m-progress`
> - `usermeta-atom-slug` replaces deprecated `usermeta-atom-linked-description`

## 🚀 Quick Start

### Installation

```bash
# Install XML Schema validation tools (required for PREMIS validation)
sudo apt install libxml2-utils

# Verify installation
xmllint --version

# Clone the repository
git clone https://github.com/penwern/curate-preservation-core.git
cd curate-preservation-core
```

### Protocol Buffers Setup

```bash
# Generate Go code from A3M protobuf definitions
buf generate

# Verify generated files
ls -la common/proto/a3m/gen/go/
```

**A3M Protobuf Repository**: https://buf.build/penwern/a3m

### Development Environment Setup

```bash
# Create required volume directories
mkdir -p /tmp/preservation/{a3m_completed,a3m_dips,working}

# A3M related directories need to be writable by A3M (uid/gid 1000)
sudo chown -R $USER:1000  /tmp/preservation/{a3m_completed,a3m_dips}
sudo chmod 775 /tmp/preservation/{a3m_completed,a3m_dips}

# Start all services (Cells, A3M, nginx)
docker compose up -d

# View service logs
docker compose logs -f preservation
```

### Configuration

```bash
# Copy and customize configuration files
cp atom_config-example.json atom_config.json

# Import example Cells Flow for testing
# Import cells/cells_flow_example.json into Pydio Cells
# The "Preserve" option will appear in right-click menus
```

## 📚 Usage

### Command Line Interface

```bash
# Basic preservation command
go run . -u admin -p personal-files/test-dir

# Multiple paths
go run . -u admin -p personal-files/dir1 -p personal-files/dir2

# Enable debug logging
CA4M_LOG_LEVEL=debug go run . -u admin -p personal-files/test-dir

# Build and run
make build
./curate-preservation-core -u admin -p personal-files/test-dir
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/preserve` | Start preservation workflow |
| `GET` | `/health` | Health check endpoint |

### API Example

```bash
# Start preservation workflow
curl -X POST http://localhost:8080/preserve \
  -H "Content-Type: application/json" \
  -d '{
    "user": "admin",
    "paths": ["personal-files/documents", "personal-files/images"]
  }'
```

## ⚙️ Configuration

### Environment Variables

All environment variables use the `CA4M_` prefix:

| Variable | Description | Default |
|----------|-------------|---------|
| `CA4M_A3M_ADDRESS` | A3M gRPC address | `localhost:7000` |
| `CA4M_A3M_COMPLETED_DIR` | A3M completed directory | `/home/a3m/.local/share/a3m/share/completed` |
| `CA4M_A3M_DIPS_DIR` | A3M dips directory | `/home/a3m/.local/share/a3m/share/dips` |
| `CA4M_CELLS_ADDRESS` | Cells address | `https://localhost:8080` |
| `CA4M_CELLS_ADMIN_TOKEN` | Cells admin token (required) | *(empty)* |
| `CA4M_CELLS_ARCHIVE_WORKSPACE` | Cells archive workspace | `common-files` |
| `CA4M_CELLS_CEC_PATH` | Cells CEC binary path | `/usr/local/bin/cec` |
| `CA4M_CLEANUP` | Clean up completed packages | `true` |
| `CA4M_ATOM_CONFIG_PATH` | Path to AtoM configuration file | `./atom_config.json` |
| `CA4M_PREMIS_ORGANIZATION` | PREMIS Agent Organization | *(empty)* |
| `CA4M_ALLOW_INSECURE_TLS` | Allow insecure TLS connections | `false` |
| `CA4M_LOG_LEVEL` | Log level (debug, info, warn, error, fatal, panic) | `info` |
| `CA4M_LOG_FILE_PATH` | Path to log file | `/var/log/curate/curate-preservation-core.log` |
| `CA4M_PROCESSING_BASE_DIR` | Base directory for processing | `/tmp/preservation` |

### Command Line Flags

```bash
./curate-preservation-core \
  --user admin \
  --path personal-files/documents \
  --path personal-files/images \
  --log-level debug \
  --cells-address https://cells.example.com \
  --a3m-address localhost:7000
```

## 🐳 Docker Deployment

### Using Docker Compose (Recommended)

```bash
# Start complete preservation stack
docker compose up -d

# View logs
docker compose logs -f preservation

# Rebuild and restart preservation service
docker compose up preservation --build -d
```

### Docker Compose Services

The development environment includes:

- **preservation** - Main preservation service (this project)
- **cells** - Pydio Cells file management
- **a3m** - Archival processing engine
- **nginx** - Reverse proxy and SSL termination

### Manual Docker Setup

```bash
# Build the image
docker build -t curate-preservation-core .

# Run with environment variables
docker run -d \
  --name preservation-core \
  -p 8080:8080 \
  -e CA4M_CELLS_ADMIN_TOKEN=your-token \
  -e CA4M_A3M_ADDRESS=a3m:7000 \
  -v preservation_data:/app/data \
  curate-preservation-core
```

## 🛠️ Development

### Environment Setup

```bash
# Clone and setup
git clone https://github.com/penwern/curate-preservation-core.git
cd curate-preservation-core

# Install dependencies
go mod download

# Build for development
make build

# Run tests
make test
```

### Available Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary |
| `make test` | Run all tests |
| `make format` | Format all Go files |
| `make lint` | Run linting |
| `make clean` | Clean build artifacts |
| `make run` | Run in development mode |

### Building

```bash
# Rebuild preservation service only
docker compose build preservation

# Rebuild and restart with new changes
docker compose up preservation --build -d

# Build for production
make build
```

### Testing

```bash
# Run all tests
make test

# Run with verbose output
go test -v ./...

# Run specific package tests
go test -v ./internal/preservation/...
```

### Code Quality

```bash
# Format code
make format

# Run linting
make lint

# Run all checks
make check
```

## 📊 Workflow States

The preservation workflow tracks the following states through Pydio Cells metadata:

| State | Description |
|-------|-------------|
| `pending` | Preservation request queued |
| `processing` | A3M processing in progress |
| `completed` | Successfully preserved |
| `failed` | Processing failed |

## 🚢 Releases

### Creating a Release

```bash
# List existing tags
git tag --list

VERTAG=v0.1.4

# Create new release tag
git tag -a $VERTAG -m $VERTAG

# Push tag to trigger CI/CD
git push origin $VERTAG

# Verify release
git describe --tags
```

## 🤝 Contributing

### Component Overview

- **Preservation Service** - Main orchestration service
- **A3M Integration** - gRPC client for archival processing
- **Cells Integration** - File management and metadata operations
- **AtoM Integration** - Optional archival description linking
- **PREMIS Generation** - Standards-compliant preservation metadata
- **API Server** - HTTP endpoints for external integration
- **CLI Interface** - Command-line interface for direct operations

### Code Standards

- Follow Go best practices and idioms
- Write clear, concise comments
- Include unit tests for new functionality
- Ensure all tests pass: `make test`
- Run linting: `make lint`
- Format code: `make format`

### Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Reporting Issues

When reporting issues, please include:

- Go version: `go version`
- Operating system and version
- A3M version and configuration
- Pydio Cells version
- Steps to reproduce the issue
- Expected vs actual behavior
- Relevant log output from all services

## 🙏 Acknowledgments

- [A3M](https://github.com/artefactual-labs/a3m) - Digital preservation processing
- [Pydio Cells](https://pydio.com/) - File management and collaboration
- [AtoM](https://www.accesstomemory.org/) - Archival description software
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [Zap](https://github.com/uber-go/zap) - High-performance logging

---

**Made with ❤️ by the Penwern team**
