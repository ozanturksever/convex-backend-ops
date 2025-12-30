package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// BackupMeta represents the meta.json for a backup
type BackupMeta struct {
	Version     string `json:"version"`
	Timestamp   string `json:"timestamp"`
	Reason      string `json:"reason"`
	FromVersion string `json:"fromVersion"`
	ToVersion   string `json:"toVersion"`
}

var (
	upgradeBundlePath string
	upgradeForce      bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Convex backend from a new bundle",
	Long:  `Upgrade Convex backend to a new version from a bundle with automatic backup.`,
	RunE:  runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().StringVarP(&upgradeBundlePath, "bundle", "b", "", "Path to the new bundle directory (required)")
	upgradeCmd.Flags().BoolVarP(&upgradeForce, "force", "f", false, "Force upgrade even if same version")
	upgradeCmd.MarkFlagRequired("bundle")
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	// Pre-flight checks
	if err := checkRoot(); err != nil {
		return err
	}

	if err := checkSystemd(); err != nil {
		return err
	}

	// Check if installed
	currentManifest, err := readManifest("/var/lib/convex/manifest.json")
	if err != nil {
		return fmt.Errorf("Convex backend is not installed. Use 'install' first")
	}

	// Validate new bundle
	if err := validateBundle(upgradeBundlePath); err != nil {
		return fmt.Errorf("invalid bundle: %w", err)
	}

	// Read new manifest
	newManifest, err := readManifest(filepath.Join(upgradeBundlePath, "manifest.json"))
	if err != nil {
		return fmt.Errorf("failed to read new manifest: %w", err)
	}

	// Compare versions
	if currentManifest.Version == newManifest.Version && !upgradeForce {
		return fmt.Errorf("already at version %s. Use --force to upgrade anyway", currentManifest.Version)
	}

	printInfo("Upgrading from v%s to v%s", currentManifest.Version, newManifest.Version)

	// Create backup
	printInfo("Creating backup...")
	backupDir := filepath.Join("/var/lib/convex/backups", "v"+currentManifest.Version)
	if err := createBackup(backupDir, currentManifest.Version, newManifest.Version); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Stop service
	printInfo("Stopping service...")
	if err := exec.Command("systemctl", "stop", "convex-backend").Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Install new version
	printInfo("Installing new version...")
	if err := installNewVersion(upgradeBundlePath); err != nil {
		// Auto-rollback on failure
		printError("Upgrade failed: %v", err)
		printInfo("Rolling back to previous version...")
		if rbErr := performRollback(backupDir); rbErr != nil {
			return fmt.Errorf("rollback also failed: %w (original error: %v)", rbErr, err)
		}
		return fmt.Errorf("upgrade failed, rolled back to v%s: %w", currentManifest.Version, err)
	}

	// Start service
	printInfo("Starting service...")
	if err := exec.Command("systemctl", "start", "convex-backend").Run(); err != nil {
		// Auto-rollback on failure
		printError("Failed to start service: %v", err)
		printInfo("Rolling back to previous version...")
		if rbErr := performRollback(backupDir); rbErr != nil {
			return fmt.Errorf("rollback also failed: %w (original error: %v)", rbErr, err)
		}
		return fmt.Errorf("service failed to start, rolled back to v%s: %w", currentManifest.Version, err)
	}

	// Health check with auto-rollback
	printInfo("Waiting for backend to be ready...")
	if err := waitForHealth("http://localhost:3210", 30*time.Second); err != nil {
		printError("Health check failed: %v", err)
		showServiceLogs()
		printInfo("Rolling back to previous version...")
		exec.Command("systemctl", "stop", "convex-backend").Run()
		if rbErr := performRollback(backupDir); rbErr != nil {
			return fmt.Errorf("rollback also failed: %w (original error: %v)", rbErr, err)
		}
		return fmt.Errorf("health check failed, rolled back to v%s: %w", currentManifest.Version, err)
	}

	// Prune old backups (only after successful upgrade)
	printInfo("Pruning old backups...")
	pruneBackups()

	printSuccess("Upgraded from v%s to v%s", currentManifest.Version, newManifest.Version)
	fmt.Println()
	fmt.Printf("Backup created: %s\n", backupDir)

	return nil
}

func createBackup(backupDir, fromVersion, toVersion string) error {
	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy binary
	if err := copyFile("/usr/local/bin/convex-backend", filepath.Join(backupDir, "convex-backend")); err != nil {
		return fmt.Errorf("failed to backup binary: %w", err)
	}

	// Copy data directory
	if err := copyDir("/var/lib/convex/data", filepath.Join(backupDir, "data")); err != nil {
		return fmt.Errorf("failed to backup data: %w", err)
	}

	// Copy manifest
	if err := copyFile("/var/lib/convex/manifest.json", filepath.Join(backupDir, "manifest.json")); err != nil {
		return fmt.Errorf("failed to backup manifest: %w", err)
	}

	// Write meta.json
	meta := BackupMeta{
		Version:     fromVersion,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Reason:      "upgrade",
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	}

	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize meta: %w", err)
	}

	if err := os.WriteFile(filepath.Join(backupDir, "meta.json"), metaData, 0644); err != nil {
		return fmt.Errorf("failed to write meta.json: %w", err)
	}

	return nil
}

func installNewVersion(bundlePath string) error {
	// Copy new binary
	if err := copyFile(filepath.Join(bundlePath, "backend"), "/usr/local/bin/convex-backend"); err != nil {
		return fmt.Errorf("failed to copy new binary: %w", err)
	}

	// Make executable
	if err := os.Chmod("/usr/local/bin/convex-backend", 0755); err != nil {
		return fmt.Errorf("failed to set binary permissions: %w", err)
	}

	// Copy new manifest
	if err := copyFile(filepath.Join(bundlePath, "manifest.json"), "/var/lib/convex/manifest.json"); err != nil {
		return fmt.Errorf("failed to copy new manifest: %w", err)
	}

	// Note: Database is preserved (not replaced during upgrade)
	return nil
}

func performRollback(backupDir string) error {
	// Copy binary back
	if err := copyFile(filepath.Join(backupDir, "convex-backend"), "/usr/local/bin/convex-backend"); err != nil {
		return fmt.Errorf("failed to restore binary: %w", err)
	}

	// Make executable
	if err := os.Chmod("/usr/local/bin/convex-backend", 0755); err != nil {
		return fmt.Errorf("failed to set binary permissions: %w", err)
	}

	// Copy data back
	if err := os.RemoveAll("/var/lib/convex/data"); err != nil {
		return fmt.Errorf("failed to remove current data: %w", err)
	}
	if err := copyDir(filepath.Join(backupDir, "data"), "/var/lib/convex/data"); err != nil {
		return fmt.Errorf("failed to restore data: %w", err)
	}

	// Copy manifest back
	if err := copyFile(filepath.Join(backupDir, "manifest.json"), "/var/lib/convex/manifest.json"); err != nil {
		return fmt.Errorf("failed to restore manifest: %w", err)
	}

	// Start service
	if err := exec.Command("systemctl", "start", "convex-backend").Run(); err != nil {
		return fmt.Errorf("failed to start service after rollback: %w", err)
	}

	return nil
}

func pruneBackups() {
	// Get retention count from environment
	retention := 3
	if envRetention := os.Getenv("CONVEX_BACKUP_RETENTION"); envRetention != "" {
		if r, err := strconv.Atoi(envRetention); err == nil && r > 0 {
			retention = r
		}
	}

	backupsDir := "/var/lib/convex/backups"
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return
	}

	// Get backup info with timestamps
	type backupInfo struct {
		path      string
		timestamp time.Time
	}

	var backups []backupInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metaPath := filepath.Join(backupsDir, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var meta BackupMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		ts, err := time.Parse(time.RFC3339, meta.Timestamp)
		if err != nil {
			continue
		}

		backups = append(backups, backupInfo{
			path:      filepath.Join(backupsDir, entry.Name()),
			timestamp: ts,
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].timestamp.After(backups[j].timestamp)
	})

	// Remove old backups
	for i := retention; i < len(backups); i++ {
		os.RemoveAll(backups[i].path)
	}
}
