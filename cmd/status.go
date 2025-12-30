package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// StatusOutput represents JSON output for status command
type StatusOutput struct {
	Installed      bool      `json:"installed"`
	Manifest       *Manifest `json:"manifest,omitempty"`
	ServiceStatus  string    `json:"serviceStatus"`
	ServiceEnabled bool      `json:"serviceEnabled"`
	Health         string    `json:"health"`
	BackendURL     string    `json:"backendUrl"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current installation status",
	Long:  `Display the current status of the Convex backend installation.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	output := StatusOutput{
		BackendURL: "http://localhost:3210",
	}

	// Check if installed (manifest exists)
	manifestPath := filepath.Join("/var/lib/convex", "manifest.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err == nil {
			output.Installed = true
			output.Manifest = &manifest
		}
	}

	// Check service status
	output.ServiceStatus = getServiceStatus()
	output.ServiceEnabled = isServiceEnabled()

	// Health check
	output.Health = checkHealth(output.BackendURL)

	if flagJSON {
		return printJSON(output)
	}

	// Human-readable output
	fmt.Println("Convex Backend Status")
	fmt.Println("=====================")
	fmt.Println()

	if !output.Installed {
		fmt.Println("Status: Not installed")
		fmt.Println()
		fmt.Println("Run 'convex-backend-ops install --bundle <path>' to install.")
		return nil
	}

	fmt.Printf("Name:           %s\n", output.Manifest.Name)
	fmt.Printf("Version:        %s\n", output.Manifest.Version)
	fmt.Printf("Service Status: %s", output.ServiceStatus)
	if output.ServiceEnabled {
		fmt.Print(" (enabled)")
	}
	fmt.Println()
	fmt.Printf("Health:         %s\n", output.Health)
	fmt.Println()

	if len(output.Manifest.Apps) > 0 {
		fmt.Println("Bundled Apps:")
		for _, app := range output.Manifest.Apps {
			fmt.Printf("  - %s\n", app)
		}
		fmt.Println()
	}

	fmt.Println("Paths:")
	fmt.Println("  Binary: /usr/local/bin/convex-backend")
	fmt.Println("  Data:   /var/lib/convex/data/")
	fmt.Println("  Config: /etc/convex/")

	return nil
}

func getServiceStatus() string {
	cmd := exec.Command("systemctl", "is-active", "convex-backend")
	output, _ := cmd.Output()
	status := string(output)
	if len(status) > 0 && status[len(status)-1] == '\n' {
		status = status[:len(status)-1]
	}
	if status == "" {
		return "unknown"
	}
	return status
}

func isServiceEnabled() bool {
	cmd := exec.Command("systemctl", "is-enabled", "convex-backend")
	output, _ := cmd.Output()
	return string(output) == "enabled\n"
}

func checkHealth(backendURL string) string {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(backendURL + "/version")
	if err != nil {
		return "unhealthy"
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "healthy"
	}
	return "unhealthy"
}
