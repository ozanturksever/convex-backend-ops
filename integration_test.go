package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestVersion_Integration tests the version command in a container
func TestVersion_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Build the binary first
	buildBinary(t)

	// Start systemd container
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	// Copy binary to container
	copyBinaryToContainer(t, ctx, container)

	// Run version command
	exitCode, output := execInContainer(t, ctx, container, []string{"/tmp/convex-backend-ops", "version"})

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "convex-backend-ops")
	assert.Contains(t, output, "Build Info:")
}

// TestStatus_NotInstalled_Integration tests status when not installed
func TestStatus_NotInstalled_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	buildBinary(t)
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	copyBinaryToContainer(t, ctx, container)

	// Run status command
	exitCode, output := execInContainer(t, ctx, container, []string{"/tmp/convex-backend-ops", "status"})

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Not installed")
}

// TestInstall_Integration tests the full install flow
func TestInstall_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Check if sample bundle has a real backend binary
	backendPath := filepath.Join("testdata", "sample-bundle", "backend")
	if _, err := os.Stat(backendPath); os.IsNotExist(err) {
		t.Skip("skipping: run ./scripts/download-backend.sh first to get a real backend binary")
	}

	ctx := context.Background()

	buildBinary(t)
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	copyBinaryToContainer(t, ctx, container)
	copyBundleToContainer(t, ctx, container)

	// Run install command
	exitCode, output := execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "install", "--bundle", "/tmp/bundle", "--yes",
	})

	assert.Equal(t, 0, exitCode, "install failed: %s", output)
	assert.Contains(t, output, "installed successfully")

	// Verify files were created
	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-f", "/usr/local/bin/convex-backend"})
	assert.Equal(t, 0, exitCode, "backend binary not installed")

	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-f", "/var/lib/convex/manifest.json"})
	assert.Equal(t, 0, exitCode, "manifest not installed")

	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-f", "/etc/convex/convex.env"})
	assert.Equal(t, 0, exitCode, "env config not created")

	// Verify service is enabled
	exitCode, _ = execInContainer(t, ctx, container, []string{"systemctl", "is-enabled", "convex-backend"})
	assert.Equal(t, 0, exitCode, "service not enabled")
}

// TestUninstall_Integration tests the uninstall flow
func TestUninstall_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	backendPath := filepath.Join("testdata", "sample-bundle", "backend")
	if _, err := os.Stat(backendPath); os.IsNotExist(err) {
		t.Skip("skipping: run ./scripts/download-backend.sh first to get a real backend binary")
	}

	ctx := context.Background()

	buildBinary(t)
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	copyBinaryToContainer(t, ctx, container)
	copyBundleToContainer(t, ctx, container)

	// Install first
	exitCode, output := execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "install", "--bundle", "/tmp/bundle", "--yes",
	})
	require.Equal(t, 0, exitCode, "install failed: %s", output)

	// Now uninstall
	exitCode, output = execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "uninstall", "--yes",
	})

	assert.Equal(t, 0, exitCode, "uninstall failed: %s", output)
	assert.Contains(t, output, "uninstalled")

	// Verify files were removed
	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-f", "/usr/local/bin/convex-backend"})
	assert.NotEqual(t, 0, exitCode, "backend binary should be removed")

	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-d", "/var/lib/convex"})
	assert.NotEqual(t, 0, exitCode, "data directory should be removed")
}

// TestListBackups_Integration tests the list-backups command
func TestListBackups_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	buildBinary(t)
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	copyBinaryToContainer(t, ctx, container)

	// Run list-backups on fresh system (no backups)
	exitCode, output := execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "list-backups",
	})

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "No backups found")
}

// TestReset_Integration tests the reset command
func TestReset_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	backendPath := filepath.Join("testdata", "sample-bundle", "backend")
	if _, err := os.Stat(backendPath); os.IsNotExist(err) {
		t.Skip("skipping: run convex-bundler first to get a real backend binary")
	}

	ctx := context.Background()

	buildBinary(t)
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	copyBinaryToContainer(t, ctx, container)
	copyBundleToContainer(t, ctx, container)

	// Install first
	exitCode, output := execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "install", "--bundle", "/tmp/bundle", "--yes",
	})
	require.Equal(t, 0, exitCode, "install failed: %s", output)

	// Create a test file in data directory to verify it gets deleted
	_, _, _ = container.Exec(ctx, []string{"touch", "/var/lib/convex/data/test-file"})

	// Run reset
	exitCode, output = execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "reset", "--yes",
	})

	assert.Equal(t, 0, exitCode, "reset failed: %s", output)
	assert.Contains(t, output, "reset complete")

	// Verify data was deleted
	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-f", "/var/lib/convex/data/test-file"})
	assert.NotEqual(t, 0, exitCode, "test file should be deleted")

	// Verify config was preserved
	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-f", "/etc/convex/convex.env"})
	assert.Equal(t, 0, exitCode, "config should be preserved")
}

// TestUpgrade_Integration tests the upgrade command with backup creation
func TestUpgrade_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	backendPath := filepath.Join("testdata", "sample-bundle", "backend")
	if _, err := os.Stat(backendPath); os.IsNotExist(err) {
		t.Skip("skipping: run convex-bundler first to get a real backend binary")
	}

	ctx := context.Background()

	buildBinary(t)
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	copyBinaryToContainer(t, ctx, container)
	copyBundleToContainer(t, ctx, container)

	// Install first
	exitCode, output := execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "install", "--bundle", "/tmp/bundle", "--yes",
	})
	require.Equal(t, 0, exitCode, "install failed: %s", output)

	// Upgrade with --force (same version)
	exitCode, output = execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "upgrade", "--bundle", "/tmp/bundle", "--yes", "--force",
	})

	assert.Equal(t, 0, exitCode, "upgrade failed: %s", output)
	assert.Contains(t, output, "Upgraded")

	// Verify backup was created
	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-d", "/var/lib/convex/backups"})
	assert.Equal(t, 0, exitCode, "backups directory should exist")

	// List backups to verify backup was created
	exitCode, output = execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "list-backups",
	})
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "backups") // Should show backup count
	assert.Contains(t, output, "upgrade") // Should show reason
}

// TestUpgrade_SameVersion_Integration tests that upgrade fails without --force for same version
func TestUpgrade_SameVersion_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	backendPath := filepath.Join("testdata", "sample-bundle", "backend")
	if _, err := os.Stat(backendPath); os.IsNotExist(err) {
		t.Skip("skipping: run convex-bundler first to get a real backend binary")
	}

	ctx := context.Background()

	buildBinary(t)
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	copyBinaryToContainer(t, ctx, container)
	copyBundleToContainer(t, ctx, container)

	// Install first
	exitCode, output := execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "install", "--bundle", "/tmp/bundle", "--yes",
	})
	require.Equal(t, 0, exitCode, "install failed: %s", output)

	// Try upgrade without --force (same version) - should fail
	exitCode, output = execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "upgrade", "--bundle", "/tmp/bundle", "--yes",
	})

	assert.NotEqual(t, 0, exitCode, "upgrade should fail for same version without --force")
	assert.Contains(t, output, "already at version")
}

// TestRollback_Integration tests the rollback command
func TestRollback_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	backendPath := filepath.Join("testdata", "sample-bundle", "backend")
	if _, err := os.Stat(backendPath); os.IsNotExist(err) {
		t.Skip("skipping: run convex-bundler first to get a real backend binary")
	}

	ctx := context.Background()

	buildBinary(t)
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	copyBinaryToContainer(t, ctx, container)
	copyBundleToContainer(t, ctx, container)

	// Install first
	exitCode, output := execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "install", "--bundle", "/tmp/bundle", "--yes",
	})
	require.Equal(t, 0, exitCode, "install failed: %s", output)

	// Create a marker file to verify data is backed up and restored
	_, _, _ = container.Exec(ctx, []string{"touch", "/var/lib/convex/data/marker-before-upgrade"})

	// Upgrade with --force to create a backup
	exitCode, output = execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "upgrade", "--bundle", "/tmp/bundle", "--yes", "--force",
	})
	require.Equal(t, 0, exitCode, "upgrade failed: %s", output)

	// Verify marker file still exists (data preserved during upgrade)
	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-f", "/var/lib/convex/data/marker-before-upgrade"})
	assert.Equal(t, 0, exitCode, "marker should exist after upgrade")

	// Delete the marker to simulate data change after upgrade
	_, _, _ = container.Exec(ctx, []string{"rm", "/var/lib/convex/data/marker-before-upgrade"})

	// Now rollback
	exitCode, output = execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "rollback", "--yes",
	})

	assert.Equal(t, 0, exitCode, "rollback failed: %s", output)
	assert.Contains(t, output, "Rolled back")

	// Verify marker file is restored (data restored from backup)
	exitCode, _ = execInContainer(t, ctx, container, []string{"test", "-f", "/var/lib/convex/data/marker-before-upgrade"})
	assert.Equal(t, 0, exitCode, "marker should be restored after rollback")

	// Verify service is still running
	exitCode, _ = execInContainer(t, ctx, container, []string{"systemctl", "is-active", "convex-backend"})
	assert.Equal(t, 0, exitCode, "service should be running after rollback")
}

// TestRollback_NoBackup_Integration tests that rollback fails gracefully when no backups exist
func TestRollback_NoBackup_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	backendPath := filepath.Join("testdata", "sample-bundle", "backend")
	if _, err := os.Stat(backendPath); os.IsNotExist(err) {
		t.Skip("skipping: run convex-bundler first to get a real backend binary")
	}

	ctx := context.Background()

	buildBinary(t)
	container := startSystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	copyBinaryToContainer(t, ctx, container)
	copyBundleToContainer(t, ctx, container)

	// Install first (no upgrade, so no backups)
	exitCode, output := execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "install", "--bundle", "/tmp/bundle", "--yes",
	})
	require.Equal(t, 0, exitCode, "install failed: %s", output)

	// Try rollback without any backups - should fail
	exitCode, output = execInContainer(t, ctx, container, []string{
		"/tmp/convex-backend-ops", "rollback", "--yes",
	})

	assert.NotEqual(t, 0, exitCode, "rollback should fail when no backups exist")
	assert.Contains(t, output, "no backups found")
}

// Helper functions

func buildBinary(t *testing.T) {
	t.Helper()
	// Build for linux/amd64 since containers are linux
	// NOTE: Binary must be pre-built before running integration tests:
	//   GOOS=linux GOARCH=amd64 go build -o convex-backend-ops .
	// This is intentionally not automated to allow faster test iteration
}

func startSystemdContainer(t *testing.T, ctx context.Context) testcontainers.Container {
	t.Helper()

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "./testdata",
			Dockerfile: "Dockerfile",
		},
		Privileged: true,
		Tmpfs: map[string]string{
			"/run":      "rw,noexec,nosuid",
			"/run/lock": "rw,noexec,nosuid",
		},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount("/sys/fs/cgroup", "/sys/fs/cgroup"),
		),
		HostConfigModifier: func(hc *container.HostConfig) {
			// Required for systemd to manage cgroups properly in containers
			hc.CgroupnsMode = "host"
		},
		WaitingFor: wait.ForExec([]string{"systemctl", "is-system-running", "--wait"}).
			WithStartupTimeout(120 * time.Second).
			WithExitCodeMatcher(func(exitCode int) bool {
				// 0 = running, 1 = degraded (acceptable for containers)
				return exitCode == 0 || exitCode == 1
			}),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	return container
}

func copyBinaryToContainer(t *testing.T, ctx context.Context, container testcontainers.Container) {
	t.Helper()

	// The binary should be built for linux/amd64
	binaryPath := "./convex-backend-ops"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("binary not found, run: GOOS=linux GOARCH=amd64 go build -o convex-backend-ops .")
	}

	err := container.CopyFileToContainer(ctx, binaryPath, "/tmp/convex-backend-ops", 0755)
	require.NoError(t, err)
}

func copyBundleToContainer(t *testing.T, ctx context.Context, container testcontainers.Container) {
	t.Helper()

	bundlePath := "./testdata/sample-bundle"
	
	// Copy each file individually since CopyDirToContainer may not work as expected
	files := []string{"backend", "convex.db", "manifest.json", "credentials.json"}
	
	// Create bundle directory
	_, _, err := container.Exec(ctx, []string{"mkdir", "-p", "/tmp/bundle/storage"})
	require.NoError(t, err)

	for _, f := range files {
		srcPath := filepath.Join(bundlePath, f)
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}
		dstPath := "/tmp/bundle/" + f
		err := container.CopyFileToContainer(ctx, srcPath, dstPath, 0644)
		require.NoError(t, err)
	}

	// Make backend executable
	_, _, err = container.Exec(ctx, []string{"chmod", "+x", "/tmp/bundle/backend"})
	require.NoError(t, err)
}

func execInContainer(t *testing.T, ctx context.Context, container testcontainers.Container, cmd []string) (int, string) {
	t.Helper()

	exitCode, reader, err := container.Exec(ctx, cmd)
	require.NoError(t, err)

	buf := make([]byte, 4096)
	n, _ := reader.Read(buf)
	output := string(buf[:n])

	return exitCode, output
}
