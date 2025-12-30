package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Completely remove Convex backend",
	Long:  `Completely remove Convex backend including all data, backups, and configuration.`,
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	// Pre-flight checks
	if err := checkRoot(); err != nil {
		return err
	}

	// Confirm action
	if !flagYes {
		fmt.Println("This will delete all Convex backend data, including:")
		fmt.Println("  - Binary: /usr/local/bin/convex-backend")
		fmt.Println("  - Data:   /var/lib/convex/ (including all backups)")
		fmt.Println("  - Config: /etc/convex/")
		fmt.Println("  - Service: convex-backend.service")
		fmt.Println()
		fmt.Print("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "yes" {
			return fmt.Errorf("uninstall cancelled")
		}
	}

	printInfo("Uninstalling Convex backend...")

	// Stop and disable service
	printInfo("Stopping service...")
	exec.Command("systemctl", "stop", "convex-backend").Run()
	exec.Command("systemctl", "disable", "convex-backend").Run()

	// Remove files
	printInfo("Removing files...")

	filesToRemove := []string{
		"/usr/local/bin/convex-backend",
		"/etc/systemd/system/convex-backend.service",
	}

	for _, f := range filesToRemove {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			printError("Failed to remove %s: %v", f, err)
		}
	}

	// Remove directories
	dirsToRemove := []string{
		"/var/lib/convex",
		"/etc/convex",
	}

	for _, d := range dirsToRemove {
		if err := os.RemoveAll(d); err != nil {
			printError("Failed to remove %s: %v", d, err)
		}
	}

	// Reload systemd
	exec.Command("systemctl", "daemon-reload").Run()

	printSuccess("Convex backend uninstalled")
	fmt.Println()
	fmt.Println("Removed:")
	fmt.Println("  - Binary: /usr/local/bin/convex-backend")
	fmt.Println("  - Data:   /var/lib/convex/")
	fmt.Println("  - Config: /etc/convex/")
	fmt.Println("  - Service: convex-backend.service")

	return nil
}
