package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Factory reset - keeps config, deletes data",
	Long: `Factory reset the Convex backend.

This will delete all database data but preserve:
  - Configuration files (/etc/convex/)
  - Backups (/var/lib/convex/backups/)
  - Admin key and instance secret`,
	RunE: runReset,
}

func init() {
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) error {
	// Pre-flight checks
	if err := checkRoot(); err != nil {
		return err
	}

	// Check if installed
	if _, err := os.Stat("/var/lib/convex/manifest.json"); os.IsNotExist(err) {
		return fmt.Errorf("Convex backend is not installed")
	}

	// Confirm action
	if !flagYes {
		fmt.Println("This will delete all database data but keep configuration.")
		fmt.Println()
		fmt.Println("Will delete:")
		fmt.Println("  - Database: /var/lib/convex/data/")
		fmt.Println()
		fmt.Println("Will preserve:")
		fmt.Println("  - Config:  /etc/convex/")
		fmt.Println("  - Backups: /var/lib/convex/backups/")
		fmt.Println()
		fmt.Print("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "yes" {
			return fmt.Errorf("reset cancelled")
		}
	}

	printInfo("Performing factory reset...")

	// Stop service
	printInfo("Stopping service...")
	if err := exec.Command("systemctl", "stop", "convex-backend").Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Delete data directory contents
	printInfo("Deleting database data...")
	dataDir := "/var/lib/convex/data"
	
	// Remove contents of data directory but keep the directory itself
	entries, err := os.ReadDir(dataDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read data directory: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(dataDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			printError("Failed to remove %s: %v", path, err)
		}
	}

	// Recreate empty data directory structure
	if err := os.MkdirAll("/var/lib/convex/data/storage", 0755); err != nil {
		return fmt.Errorf("failed to recreate data directory: %w", err)
	}

	// Start service
	printInfo("Starting service...")
	if err := exec.Command("systemctl", "start", "convex-backend").Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	printSuccess("Factory reset complete")
	fmt.Println()
	fmt.Println("Deleted:")
	fmt.Println("  - Database: /var/lib/convex/data/")
	fmt.Println()
	fmt.Println("Preserved:")
	fmt.Println("  - Config:  /etc/convex/")
	fmt.Println("  - Backups: /var/lib/convex/backups/")

	return nil
}
