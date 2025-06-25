# Curate Preservation Core

A digital preservation service that integrates with Penwern A3M archival processing and Pydio Cells file management for comprehensive digital archival workflows. Also known as CA4M.

## Features

- **Automated Preservation Workflows** - Seamless integration with A3M for archival processing
- **Metadata Management** - Tracks preservation status through Pydio Cells metadata
- **RESTful API** - HTTP endpoints for preservation operations
- **Command Line Interface** - Direct CLI access for administrative tasks
- **Docker Support** - Containerized deployment with development environment

## Architecture

The service acts as a bridge between:
- **Pydio Cells** (file management and metadata storage)
- **A3M/A4M** (archival processing engine)
- **AtoM** (archival description and access)

## Dependencies

### Required
- **Penwern A3M (A4M)** - Shared file system required for processing
- **Cells Enterprise Client Binary (CEC)** - For Pydio Cells integration
- **Pydio Cells** with configured metadata namespaces

### Metadata Namespaces
- `usermeta-preservation-status` (required) - Tracks preservation workflow status
- `usermeta-dip-status` (optional) - Dissemination Information Package status
- `usermeta-atom-slug` (optional) - AtoM archival description linking

> **Important**: Metadata namespaces must be editable by users. Admin users cannot edit personal file tags.

> **Migration Notes**: 
> - `usermeta-preservation-status` replaces deprecated `usermeta-a3m-progress`
> - `usermeta-atom-slug` replaces deprecated `usermeta-atom-linked-description`

## Setup

### System Requirements

```bash
# Install XML Schema validation tools (required for PREMIS validation)
sudo apt install libxml2-utils

# Verify installation
xmllint --version
```

### Protocol Buffers

The service uses gRPC communication with A3M via Protocol Buffers. Generate Go code using [Buf](https://buf.build/docs/installation):

```bash
# Generate Go code from A3M protobuf definitions
buf generate

# Verify generated files
ls -la common/proto/a3m/gen/go/
```

**A3M Protobuf Repository**: https://buf.build/penwern/a3m

## Development

### Environment Setup

```bash
# Create required volume directories
mkdir -p /tmp/preservation/{a3m_completed,a3m_dips,working}

# Start all services (Cells, A3M, nginx)
docker compose up -d

# View service logs
docker compose logs -f preservation
```

### Building

```bash
# Rebuild preservation service only
docker compose build preservation

# Rebuild and restart with new changes
docker compose up preservation --build -d

# Build for production
make build
```

### Configuration

Copy and customize the configuration files:

```bash
# Atom configuration
cp atom_config-example.json atom_config.json
```

Import the example Cells Flow:
```bash
cells/cells_flow_example.json
```

## Usage

### Command Line Interface

```bash
# Basic preservation command
go run . -u admin -p personal-files/test-dir

# Multiple paths
go run . -u admin -p personal-files/dir1,personal-files/dir2

# Enable debug logging via environment variable
LOG_LEVEL=debug go run . -u admin -p personal-files/test-dir
```

### HTTP API

**Start Preservation:**
```bash
curl -X POST http://localhost:6905/preserve \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "paths": ["personal-files/test-dir"],
    "options": {
      "processing_config": "default"
    }
  }'
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/preserve` | Start preservation workflow |

## Configuration

### Environment Variables

All environment variables use the `CA4M_` prefix.

| Variable | Description | Default |
|----------|-------------|---------|
| `CA4M_A3M_ADDRESS` | A3M gRPC address | `localhost:7000` |
| `CA4M_A3M_COMPLETED_DIR` | A3M completed directory | `/home/a3m/.local/share/a3m/share/completed` |
| `CA4M_A3M_DIPS_DIR` | A3M dips directory | `/home/a3m/.local/share/a3m/share/dips` |
| `CA4M_CELLS_ADDRESS` | Cells address | `https://localhost:8080` |
| `CA4M_CELLS_ADMIN_TOKEN` | Cells admin token (required) | *(empty)* |
| `CA4M_CELLS_ARCHIVE_WORKSPACE` | Cells archive workspace | `common-files` |
| `CA4M_CELLS_CEC_PATH` | Cells CEC binary path | `/usr/local/bin/cec` |
| `CA4M_ATOM_CONFIG_PATH` | Path to AtoM configuration file | `./atom_config.json` |
| `CA4M_PREMIS_ORGANIZATION` | PREMIS Agent Organization | *(empty)* |
| `CA4M_ALLOW_INSECURE_TLS` | Allow insecure TLS connections | `false` |
| `CA4M_LOG_LEVEL` | Log level (debug, info, warn, error, fatal, panic) | `info` |
| `CA4M_LOG_FILE_PATH` | Path to log file | `/var/log/curate/curate-preservation-core.log` |
| `CA4M_PROCESSING_BASE_DIR` | Base directory for processing | `/tmp/preservation` |

### Docker Compose Services

- **preservation** - Main preservation service
- **cells** - Pydio Cells file management
- **a3m** - Archival processing engine
- **nginx** - Reverse proxy and SSL termination

## Monitoring & Logs

```bash
# View all service logs
docker compose logs -f

# View specific service logs
docker compose logs -f preservation
docker compose logs -f a3m
```

## Troubleshooting

### Common Issues

**A3M Connection Errors:**
```bash
# Check A3M service status
docker compose ps a3m
docker compose logs a3m

# Verify gRPC connectivity
grpcurl -plaintext localhost:7000 list
```

**Cells Metadata Issues:**
- Ensure metadata namespaces are configured in Cells admin panel
- Verify user permissions for metadata editing
- Check Cells API connectivity

**File System Permissions:**
```bash
# Fix volume permissions
sudo chown -R $USER:$USER /tmp/preservation/
chmod -R 755 /tmp/preservation/
```

### Debug Mode

```bash
# Run with debug logging
CA4M_LOG_LEVEL=debug go run . -u admin -p personal-files/test-dir
```

## Releases

### Creating a Release

```bash
# List existing tags
git tag --list

# Create new release tag
git tag -a v0.1.5 -m "Release version 0.1.5"

# Push tag to trigger CI/CD
git push origin v0.1.5

# Verify release
git describe --tags
```

### Version Information

```bash
# Check current version
curate-preservation-core version
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
