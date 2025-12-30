# convex-backend-ops

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/ozanturksever/convex-backend-ops)](https://github.com/ozanturksever/convex-backend-ops/releases)

Single-binary operations tool for deploying and managing Convex backend with pre-deployed apps. Can be deployed on air-gapped or restricted network environments without needing Node.js.

## Features

- **Single Binary** - One file that installs, upgrades, and manages everything
- **Pre-deployed Apps** - Apps are bundled and ready to use immediately after installation
- **Upgrade Support** - In-place upgrades with automatic backups
- **Auto-Rollback** - Automatically rolls back failed upgrades
- **No Dependencies** - Target system only needs Linux + systemd

## Installation

### From GitHub Releases

Download the latest release for your platform:

```bash
# Linux AMD64
curl -fsSL https://github.com/ozanturksever/convex-backend-ops/releases/latest/download/convex-backend-ops_Linux_x86_64.tar.gz | tar xz

# Linux ARM64
curl -fsSL https://github.com/ozanturksever/convex-backend-ops/releases/latest/download/convex-backend-ops_Linux_arm64.tar.gz | tar xz

# macOS AMD64
curl -fsSL https://github.com/ozanturksever/convex-backend-ops/releases/latest/download/convex-backend-ops_Darwin_x86_64.tar.gz | tar xz

# macOS ARM64 (Apple Silicon)
curl -fsSL https://github.com/ozanturksever/convex-backend-ops/releases/latest/download/convex-backend-ops_Darwin_arm64.tar.gz | tar xz
```

### From Source

```bash
go install github.com/ozanturksever/convex-backend-ops@latest
```

## Usage

First, create a bundle using [convex-bundler](https://github.com/ozanturksever/convex-bundler):

```bash
convex-bundler --app ./convex --output ./bundle --name "My Backend"
```

Then use `convex-backend-ops` to manage the installation:

### Install

```bash
sudo ./convex-backend-ops install --bundle ./bundle
```

### Check Status

```bash
sudo ./convex-backend-ops status
```

### Upgrade

```bash
sudo ./convex-backend-ops upgrade --bundle ./new-bundle
```

### Rollback

```bash
# Rollback to most recent backup
sudo ./convex-backend-ops rollback

# Rollback to specific version
sudo ./convex-backend-ops rollback 1.2.3
```

### List Backups

```bash
sudo ./convex-backend-ops list-backups
```

### Factory Reset

```bash
sudo ./convex-backend-ops reset
```

### Uninstall

```bash
sudo ./convex-backend-ops uninstall
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--yes` | `-y` | Skip all confirmation prompts |
| `--quiet` | `-q` | Suppress non-essential output |
| `--json` | | Output results in JSON format |

## Directory Structure

After installation, the following structure is created on the target system:

```
/usr/local/bin/
  convex-backend              # Convex backend binary

/var/lib/convex/
  data/
    convex.db                 # SQLite database
    storage/                  # File storage
  manifest.json               # Installed version metadata
  backups/                    # Automatic backups from upgrades

/etc/convex/
  convex.env                  # Environment configuration
  admin.key                   # Admin key
  instance.secret             # Instance secret

/etc/systemd/system/
  convex-backend.service      # systemd service unit
```

## Development

### Prerequisites

- Go 1.21+
- Docker (for integration tests)
- [convex-bundler](https://github.com/ozanturksever/convex-bundler) (for creating test bundles)

### Build

```bash
# Build for current platform
make build

# Build for all platforms
make build-all
```

### Test

```bash
# Run all tests (requires Docker)
make test

# Run unit tests only
make test-short

# Run integration tests only
make test-integration
```

### Release

```bash
# Create release artifacts
make release

# Or use goreleaser
goreleaser release --snapshot --clean
```

## Requirements

### Target Machine

- Linux (Ubuntu 22.04+ recommended)
- systemd
- No other dependencies required!

## License

Apache-2.0 - See [LICENSE](LICENSE) for details.
