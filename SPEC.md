# convex-backend-ops

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)

Single-binary operations tool for deploying and managing Convex backend with pre-deployed apps. Can be deployed on air-gapped or restricted network environments without needing Node.js.

## Installation

```bash
go install github.com/ozanturksever/convex-backend-ops@latest
```

Or download pre-built binaries from the [releases page](https://github.com/ozanturksever/convex-backend-ops/releases).

## Features

- **Single Binary Installer** - One file that installs everything
- **Pre-deployed Apps** - Apps are bundled and ready to use immediately after installation
- **Upgrade Support** - In-place upgrades with automatic backups
- **Auto-Rollback** - Automatically rolls back failed upgrades
- **No Dependencies** - Target system only needs Linux + systemd

## Directory Structure

After installation, the following directory structure is created:

```
/usr/local/bin/
  convex-backend              # Convex backend binary

/var/lib/convex/
  data/
    convex.db                 # SQLite database (from bundle)
    storage/                  # File storage (from bundle)
  manifest.json               # Installed version metadata (from bundle)
  backups/
    v1.2.3/                   # Backup from upgrade
      convex-backend          # Previous binary
      data/                   # Previous database snapshot
      manifest.json           # Previous manifest
      meta.json               # Backup metadata (timestamp, reason)

/etc/convex/
  convex.env                  # Environment configuration
  admin.key                   # Admin key (generated on first install)
  instance.secret             # Instance secret (generated on first install)

/etc/systemd/system/
  convex-backend.service      # systemd service unit
```

---

## Global Flags

All commands support these flags for non-interactive/scripted usage:

| Flag | Description |
|------|-------------|
| `--yes`, `-y` | Skip all confirmation prompts (assume yes) |
| `--quiet`, `-q` | Suppress non-essential output |
| `--json` | Output results in JSON format (for scripting) |

Example:
```bash
# Non-interactive uninstall
sudo ./convex-backend-ops uninstall --yes

# JSON output for scripting
./convex-backend-ops status --json

# Quiet mode for scripts
sudo ./convex-backend-ops install --yes --quiet
```

---

## Commands

### `install`

Performs fresh installation of Convex backend from a bundle.

```bash
sudo ./convex-backend-ops install --bundle /path/to/bundle
```

**Flags:**

| Flag | Short | Description | Required |
|------|-------|-------------|----------|
| `--bundle` | `-b` | Path to the bundle directory | Yes |

**Implementation Steps:**

1. **Pre-flight checks**
   - Verify running as root
   - Check systemd is available
   - Verify target directories are writable
   - Check if already installed (abort if yes, suggest `upgrade`)

2. **Extract bundle assets**
   - Copy `backend` binary from bundle → `/usr/local/bin/convex-backend`
   - Copy `convex.db` from bundle → `/var/lib/convex/data/convex.db`
   - Copy `storage/` from bundle → `/var/lib/convex/data/storage/`
   - Set binary permissions: `chmod 755`

3. **Create directory structure**
   - Create `/var/lib/convex/` with mode `0755`
   - Create `/var/lib/convex/data/` with mode `0755`
   - Create `/var/lib/convex/backups/` with mode `0755`
   - Create `/etc/convex/` with mode `0755`

4. **Extract credentials from bundle**
   - Read `credentials.json` from bundle
   - Write admin key → `/etc/convex/admin.key`
   - Write instance secret → `/etc/convex/instance.secret`
   - Set permissions: `chmod 600`

5. **Create environment config**
   - Write `/etc/convex/convex.env`:
     ```env
     CONVEX_SITE_URL=http://localhost:3210
     CONVEX_LOCAL_STORAGE=/var/lib/convex/data
     CONVEX_ADMIN_KEY_FILE=/etc/convex/admin.key
     CONVEX_INSTANCE_SECRET_FILE=/etc/convex/instance.secret
     ```

6. **Install systemd service**
   - Write `/etc/systemd/system/convex-backend.service`:
     ```ini
     [Unit]
     Description=Convex Backend
     After=network.target

     [Service]
     Type=simple
     EnvironmentFile=/etc/convex/convex.env
     ExecStart=/usr/local/bin/convex-backend
     Restart=always
     RestartSec=5

     [Install]
     WantedBy=multi-user.target
     ```
   - Run `systemctl daemon-reload`

7. **Write manifest**
   - Copy `manifest.json` from bundle → `/var/lib/convex/manifest.json`

8. **Start service**
   - Run `systemctl enable convex-backend`
   - Run `systemctl start convex-backend`

9. **Health check**
   - Wait up to 30 seconds for `http://localhost:3210/version` to respond
   - If fails, show logs and exit with error

10. **Display success message**
    - Show backend URL, admin key, bundled apps

---

### `status`

Shows current installation status.

```bash
sudo ./convex-backend-ops status
```

**Implementation Steps:**

1. **Check installation**
   - Verify `/var/lib/convex/manifest.json` exists
   - Read manifest for version info

2. **Check service status**
   - Run `systemctl is-active convex-backend`
   - Run `systemctl is-enabled convex-backend`

3. **Health check**
   - HTTP GET `http://localhost:3210/version`
   - Parse response for backend version

4. **Display status**
   ```
   Convex Backend Status
   =====================
   
   Installed Version: 1.5.0
   Backend Version:   0.1.0
   Service Status:    running (enabled)
   Health:            healthy
   Uptime:            2d 5h 30m
   
   Bundled Apps:
     healthCheck  v1.2.3
     relay        v2.0.0
   
   Paths:
     Binary:    /usr/local/bin/convex-backend
     Data:      /var/lib/convex/data/
     Config:    /etc/convex/
   ```

---

### `upgrade`

Upgrades to a new version from a bundle with automatic backup.

```bash
sudo ./convex-backend-ops upgrade --bundle /path/to/new-bundle
```

**Flags:**

| Flag | Short | Description | Required |
|------|-------|-------------|----------|
| `--bundle` | `-b` | Path to the new bundle directory | Yes |
| `--force` | `-f` | Force upgrade even if same version | No |

**Implementation Steps:**

1. **Pre-flight checks**
   - Verify currently installed (read manifest)
   - Compare versions (embedded vs installed)
   - Abort if same version (unless `--force`)

2. **Create backup**
   - Stop service: `systemctl stop convex-backend`
   - Create backup directory: `/var/lib/convex/backups/v{current_version}/`
   - Copy binary: `convex-backend`
   - Copy database: `data/`
   - Copy manifest: `manifest.json`
   - Write `meta.json`:
     ```json
     {
       "version": "1.2.3",
       "timestamp": "2024-01-15T10:30:00Z",
       "reason": "upgrade",
       "fromVersion": "1.2.3",
       "toVersion": "1.5.0"
     }
     ```

3. **Install new version from bundle**
   - Copy `backend` binary from bundle → `/usr/local/bin/convex-backend`
   - Copy `manifest.json` from bundle → `/var/lib/convex/manifest.json`
   - Note: Database is preserved (not replaced during upgrade)

4. **Start service**
   - Run `systemctl start convex-backend`

5. **Health check with auto-rollback**
   - Wait up to 30 seconds for health check
   - If fails:
     - Log error
     - Call rollback procedure
     - Exit with error

6. **Prune old backups** (only after successful health check)
   - Keep last 3 backups (configurable via env var `CONVEX_BACKUP_RETENTION`)
   - Delete oldest backups

7. **Display success**
   ```
   ✓ Upgraded from v1.2.3 to v1.5.0
   
   Backup created: /var/lib/convex/backups/v1.2.3/
   ```

---

### `rollback [version]`

Rolls back to a previous version from backup.

```bash
# Rollback to most recent backup
sudo ./convex-backend-ops rollback

# Rollback to specific version
sudo ./convex-backend-ops rollback 1.2.3
```

**Implementation Steps:**

1. **Find backup**
   - If version specified: look for `/var/lib/convex/backups/v{version}/`
   - If no version: find most recent backup by `meta.json` timestamp
   - Abort if no backup found

2. **Stop service**
   - Run `systemctl stop convex-backend`

3. **Restore from backup**
   - Copy binary: `backups/v{version}/convex-backend` → `/usr/local/bin/convex-backend`
   - Copy database: `backups/v{version}/data/` → `/var/lib/convex/data/`
   - Copy manifest: `backups/v{version}/manifest.json` → `/var/lib/convex/manifest.json`

4. **Start service**
   - Run `systemctl start convex-backend`

5. **Health check**
   - Wait for health check to pass
   - If fails, show error (no further auto-rollback)

6. **Display success**
   ```
   ✓ Rolled back to v1.2.3
   
   Service restarted successfully.
   ```

---

### `list-backups`

Lists all available backups.

```bash
sudo ./convex-backend-ops list-backups
```

**Implementation Steps:**

1. **Scan backup directory**
   - Read `/var/lib/convex/backups/`
   - For each subdirectory, read `meta.json`

2. **Display backups**
   ```
   Available Backups
   =================
   
   VERSION   CREATED              SIZE      REASON
   v1.4.0    2024-01-14 08:15:00  125 MB    upgrade
   v1.3.0    2024-01-10 14:30:00  118 MB    upgrade
   v1.2.3    2024-01-05 09:00:00  112 MB    manual
   
   Total: 3 backups (355 MB)
   ```

---

### `version`

Shows version information.

```bash
./convex-backend-ops version
```

**Implementation Steps:**

1. **Read embedded manifest**
   - Display installer version, build timestamp, git commit

2. **Read installed manifest** (if installed)
   - Display installed version, Convex backend version

3. **Display output**
   ```
   convex-backend-ops v1.5.0
   
   Build Info:
     Timestamp:  2024-01-15T10:30:00Z
     Git Commit: abc1234
     Platform:   linux-x64
   
   Installed:
     Version:         1.5.0
     Backend Version: 0.1.0
   
   Bundled Apps:
     healthCheck  v1.2.3  (15 functions)
     relay        v2.0.0  (8 functions)
   ```

---

### `uninstall`

Completely removes Convex backend.

```bash
sudo ./convex-backend-ops uninstall
```

**Implementation Steps:**

1. **Confirm action** (skipped if `--yes` flag)
   - Prompt: "This will delete all data. Type 'yes' to confirm:"
   - Abort if not confirmed

2. **Stop and disable service**
   - Run `systemctl stop convex-backend`
   - Run `systemctl disable convex-backend`

3. **Remove files**
   - Delete `/usr/local/bin/convex-backend`
   - Delete `/var/lib/convex/` (entire directory including backups)
   - Delete `/etc/convex/` (entire directory)
   - Delete `/etc/systemd/system/convex-backend.service`

4. **Reload systemd**
   - Run `systemctl daemon-reload`

5. **Display success**
   ```
   ✓ Convex backend uninstalled
   
   Removed:
     - Binary: /usr/local/bin/convex-backend
     - Data:   /var/lib/convex/
     - Config: /etc/convex/
     - Service: convex-backend.service
   ```

---

### `reset`

Factory reset - keeps config, deletes data.

```bash
sudo ./convex-backend-ops reset
```

**Implementation Steps:**

1. **Confirm action** (skipped if `--yes` flag)
   - Prompt: "This will delete all database data but keep configuration. Type 'yes' to confirm:"
   - Abort if not confirmed

2. **Stop service**
   - Run `systemctl stop convex-backend`

3. **Delete data only**
   - Delete `/var/lib/convex/data/*` (database files)
   - Keep `/var/lib/convex/backups/` (preserve backups)
   - Keep `/etc/convex/*` (preserve config and secrets)

4. **Start service**
   - Run `systemctl start convex-backend`

5. **Display success**
   ```
   ✓ Factory reset complete
   
   Deleted:
     - Database: /var/lib/convex/data/
   
   Preserved:
     - Config:  /etc/convex/
     - Backups: /var/lib/convex/backups/
   ```

---

## Input Bundle Format

This tool expects a bundle directory created by [convex-bundler](https://github.com/ozanturksever/convex-bundler).

### Bundle Directory Structure

```
bundle/
  backend             # The convex-local-backend binary (executable)
  convex.db           # Pre-initialized SQLite database with deployed apps
  storage/            # File storage directory
  manifest.json       # Bundle metadata
  credentials.json    # Pre-generated admin credentials
```

### manifest.json

```json
{
  "name": "My Backend",
  "version": "1.0.0",
  "apps": [
    "./path/to/app1",
    "./path/to/app2"
  ],
  "platform": "linux-x64",
  "createdAt": "2024-01-15T10:30:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name for the backend |
| `version` | string | Semantic version of the bundle |
| `apps` | string[] | List of app paths that were bundled |
| `platform` | string | Target platform (`linux-x64`, `linux-arm64`) |
| `createdAt` | string | ISO 8601 timestamp of bundle creation |

### credentials.json

```json
{
  "adminKey": "convex_admin_...",
  "instanceSecret": "hex-encoded-secret"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `adminKey` | string | Pre-generated admin key for API access |
| `instanceSecret` | string | Instance secret used for key generation |

---

## Testing

Tests use Docker and [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to run integration tests in isolated Linux environments with systemd.

### Test Dockerfile

The test container is based on Ubuntu with systemd enabled:

```dockerfile
# testdata/Dockerfile
FROM ubuntu:24.04

# Install systemd and required packages
RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        systemd \
        systemd-sysv \
        systemd-resolved \
        curl \
        iproute2 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Configure systemd for container environment (remove unnecessary services)
RUN cd /lib/systemd/system/sysinit.target.wants/ && \
    ls | grep -v systemd-tmpfiles-setup | xargs rm -f && \
    rm -f /lib/systemd/system/multi-user.target.wants/* && \
    rm -f /etc/systemd/system/*.wants/* && \
    rm -f /lib/systemd/system/local-fs.target.wants/* && \
    rm -f /lib/systemd/system/sockets.target.wants/*udev* && \
    rm -f /lib/systemd/system/sockets.target.wants/*initctl* && \
    rm -f /lib/systemd/system/basic.target.wants/* && \
    rm -f /lib/systemd/system/anaconda.target.wants/* || true

CMD ["/lib/systemd/systemd"]
```

### Running Tests

```bash
# Run all tests (requires Docker)
go test ./...

# Run unit tests only (no Docker)
go test -short ./...

# Run integration tests with verbose output
go test -v -run Integration ./...
```

### Test Structure

```
pkg/
  install/
    install_test.go         # Unit tests
    install_integration_test.go  # Container tests
  upgrade/
    upgrade_test.go
    upgrade_integration_test.go
  ...
testdata/
  Dockerfile              # Test container image
  sample-bundle/          # Sample bundle for testing
    backend
    convex.db
    storage/
    manifest.json
    credentials.json
```

### Integration Test Example

```go
func TestInstall_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    ctx := context.Background()

    // Build test container with systemd
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            FromDockerfile: testcontainers.FromDockerfile{
                Context:    "./testdata",
                Dockerfile: "Dockerfile",
            },
            Privileged:   true,    // Required for systemd
            CgroupnsMode: "host", // Required for systemd cgroup management
            Tmpfs: map[string]string{
                "/run":      "rw,noexec,nosuid",
                "/run/lock": "rw,noexec,nosuid",
            },
            Mounts: testcontainers.Mounts(
                testcontainers.BindMount("/sys/fs/cgroup", "/sys/fs/cgroup"),
            ),
            ExposedPorts: []string{"3210/tcp"},
            WaitingFor:   wait.ForExec([]string{"systemctl", "is-system-running", "--wait"}).WithExitCodeMatcher(func(exitCode int) bool {
                return exitCode == 0 || exitCode == 1 // 1 = degraded but running
            }),
        },
        Started: true,
    })
    require.NoError(t, err)
    defer container.Terminate(ctx)

    // Copy bundle and binary to container
    err = container.CopyFileToContainer(ctx, "./testdata/sample-bundle", "/tmp/bundle", 0755)
    require.NoError(t, err)

    // Run install command
    exitCode, output, err := container.Exec(ctx, []string{
        "/tmp/convex-backend-ops", "install", "--bundle", "/tmp/bundle", "--yes",
    })
    require.NoError(t, err)
    assert.Equal(t, 0, exitCode)
    assert.Contains(t, output, "installed successfully")

    // Verify service is running
    exitCode, _, err = container.Exec(ctx, []string{
        "systemctl", "is-active", "convex-backend",
    })
    require.NoError(t, err)
    assert.Equal(t, 0, exitCode)
}
```

---

## Requirements

### Target Machine

- Linux (Ubuntu 22.04+ recommended)
- systemd
- No other dependencies required!

## License

Apache-2.0 - See [LICENSE](LICENSE) for details.
