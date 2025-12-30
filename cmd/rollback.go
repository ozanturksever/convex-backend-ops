package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [version]",
	Short: "Rollback to a previous version from backup",
	Long: `Rollback to a previous version from backup.

If no version is specified, rolls back to the most recent backup.
If a version is specified, rolls back to that specific version.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRollback,
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}

func runRollback(cmd *cobra.Command, args []string) error {
	// Pre-flight checks
	if err := checkRoot(); err != nil {
		return err
	}

	if err := checkSystemd(); err != nil {
		return err
	}

	// Find backup
	var backupDir string
	var backupVersion string

	if len(args) > 0 {
		// Specific version requested
		backupVersion = args[0]
		backupDir = filepath.Join("/var/lib/convex/backups", "v"+backupVersion)
		if _, err := os.Stat(backupDir); os.IsNotExist(err) {
			return fmt.Errorf("backup for version %s not found", backupVersion)
		}
	} else {
		// Find most recent backup
		var err error
		backupDir, backupVersion, err = findMostRecentBackup()
		if err != nil {
			return err
		}
	}

	printInfo("Rolling back to v%s...", backupVersion)

	// Stop service
	printInfo("Stopping service...")
	exec.Command("systemctl", "stop", "convex-backend").Run()

	// Perform rollback
	printInfo("Restoring from backup...")
	if err := restoreFromBackup(backupDir); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	// Start service
	printInfo("Starting service...")
	if err := exec.Command("systemctl", "start", "convex-backend").Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Health check
	printInfo("Waiting for backend to be ready...")
	if err := waitForHealth("http://localhost:3210", 30*time.Second); err != nil {
		showServiceLogs()
		return fmt.Errorf("health check failed after rollback: %w", err)
	}

	printSuccess("Rolled back to v%s", backupVersion)
	fmt.Println()
	fmt.Println("Service restarted successfully.")

	return nil
}

func findMostRecentBackup() (string, string, error) {
	backupsDir := "/var/lib/convex/backups"
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return "", "", fmt.Errorf("no backups found")
	}

	type backupInfo struct {
		path      string
		version   string
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
			version:   meta.Version,
			timestamp: ts,
		})
	}

	if len(backups) == 0 {
		return "", "", fmt.Errorf("no backups found")
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].timestamp.After(backups[j].timestamp)
	})

	return backups[0].path, backups[0].version, nil
}

func restoreFromBackup(backupDir string) error {
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

	return nil
}
