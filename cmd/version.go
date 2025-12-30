package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Manifest represents the installed manifest.json
type Manifest struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Apps      []string `json:"apps"`
	Platform  string   `json:"platform"`
	CreatedAt string   `json:"createdAt"`
}

// VersionOutput represents JSON output for version command
type VersionOutput struct {
	Version   string    `json:"version"`
	GitCommit string    `json:"gitCommit"`
	BuildTime string    `json:"buildTime"`
	Installed *Manifest `json:"installed,omitempty"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version information for convex-backend-ops and any installed backend.`,
	RunE:  runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) error {
	output := VersionOutput{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
	}

	// Try to read installed manifest
	manifestPath := filepath.Join("/var/lib/convex", "manifest.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err == nil {
			output.Installed = &manifest
		}
	}

	if flagJSON {
		return printJSON(output)
	}

	// Human-readable output
	fmt.Printf("convex-backend-ops %s\n", Version)
	fmt.Println()
	fmt.Println("Build Info:")
	fmt.Printf("  Git Commit: %s\n", GitCommit)
	fmt.Printf("  Build Time: %s\n", BuildTime)

	if output.Installed != nil {
		fmt.Println()
		fmt.Println("Installed:")
		fmt.Printf("  Name:    %s\n", output.Installed.Name)
		fmt.Printf("  Version: %s\n", output.Installed.Version)
		if len(output.Installed.Apps) > 0 {
			fmt.Println("  Apps:")
			for _, app := range output.Installed.Apps {
				fmt.Printf("    - %s\n", app)
			}
		}
	} else {
		fmt.Println()
		fmt.Println("Installed: (not installed)")
	}

	return nil
}

func printJSON(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
