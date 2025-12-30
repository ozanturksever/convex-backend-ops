package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ozanturksever/convex-bundler/pkg/selfhost"
	"github.com/spf13/cobra"
)

// SelfHost detection result cached at startup
var (
	isSelfHostMode bool
	selfHostOffset int64
)

// InfoOutput represents JSON output for info command
type InfoOutput struct {
	IsSelfHost     bool     `json:"isSelfHost"`
	OpsVersion     string   `json:"opsVersion,omitempty"`
	BundleName     string   `json:"bundleName,omitempty"`
	BundleVersion  string   `json:"bundleVersion,omitempty"`
	Platform       string   `json:"platform,omitempty"`
	CreatedAt      string   `json:"createdAt,omitempty"`
	Apps           []string `json:"apps,omitempty"`
	BundleSize     int64    `json:"bundleSize,omitempty"`
	Compression    string   `json:"compression,omitempty"`
	BundleChecksum string   `json:"bundleChecksum,omitempty"`
}

// VerifyOutput represents JSON output for verify command
type VerifyOutput struct {
	Valid            bool   `json:"valid"`
	ExpectedChecksum string `json:"expectedChecksum"`
	ActualChecksum   string `json:"actualChecksum"`
}

var extractOutputPath string

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract embedded bundle to a directory",
	Long: `Extract the embedded bundle from this self-extracting executable to a directory.

This command only works when the executable contains an embedded bundle
(created by convex-bundler selfhost command).`,
	RunE: runExtract,
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display embedded bundle information",
	Long: `Display information about the embedded bundle without extracting it.

This command shows metadata including bundle name, version, platform,
bundled apps, and checksum information.`,
	RunE: runInfo,
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify embedded bundle integrity",
	Long: `Verify the integrity of the embedded bundle by checking its SHA256 checksum.

This ensures the bundle has not been corrupted or tampered with.`,
	RunE: runVerify,
}

func init() {
	// Add selfhost commands
	rootCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(verifyCmd)

	// Extract command flags
	extractCmd.Flags().StringVarP(&extractOutputPath, "output", "o", "", "Output directory for extracted bundle (required)")
	extractCmd.MarkFlagRequired("output")
}

// InitSelfHostMode detects if the current executable is a self-host bundle
// This should be called during initialization
func InitSelfHostMode() {
	result, err := selfhost.DetectSelfHostMode()
	if err != nil {
		// Silently ignore detection errors - just means we're not in selfhost mode
		isSelfHostMode = false
		return
	}
	isSelfHostMode = result.IsSelfHost
	selfHostOffset = result.Offset
}

// IsSelfHostMode returns true if the current executable contains an embedded bundle
func IsSelfHostMode() bool {
	return isSelfHostMode
}

// GetEmbeddedBundlePath extracts the embedded bundle to a temp directory and returns the path
// The caller is responsible for cleaning up the temp directory
func GetEmbeddedBundlePath() (string, func(), error) {
	if !isSelfHostMode {
		return "", nil, fmt.Errorf("not running as a self-host executable")
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "convex-bundle-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// Extract bundle
	_, err = selfhost.Extract(selfhost.ExtractOptions{
		OutputDir: tempDir,
	})
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to extract embedded bundle: %w", err)
	}

	return tempDir, cleanup, nil
}

func runExtract(cmd *cobra.Command, args []string) error {
	if !isSelfHostMode {
		return fmt.Errorf("this executable does not contain an embedded bundle. Use --bundle flag to specify a bundle directory")
	}

	printInfo("Extracting embedded bundle to: %s", extractOutputPath)

	// Verify first
	printInfo("Verifying bundle integrity...")
	verifyResult, err := selfhost.Verify("")
	if err != nil {
		return fmt.Errorf("failed to verify bundle: %w", err)
	}

	if !verifyResult.Valid {
		printError("Bundle integrity check failed!")
		printError("Expected checksum: %s", verifyResult.ExpectedChecksum)
		printError("Actual checksum:   %s", verifyResult.ActualChecksum)
		os.Exit(selfhost.ExitVerificationFailed)
	}

	printInfo("Bundle integrity verified")

	// Extract
	header, err := selfhost.Extract(selfhost.ExtractOptions{
		OutputDir:  extractOutputPath,
		SkipVerify: true, // Already verified above
	})
	if err != nil {
		printError("Extraction failed: %v", err)
		os.Exit(selfhost.ExitExtractionFailed)
	}

	printSuccess("Bundle extracted successfully!")
	fmt.Println()
	fmt.Printf("Bundle Name:    %s\n", header.Manifest.Name)
	fmt.Printf("Bundle Version: %s\n", header.Manifest.Version)
	fmt.Printf("Output:         %s\n", extractOutputPath)
	fmt.Println()
	fmt.Println("Contents:")
	fmt.Println("  - backend (executable)")
	fmt.Println("  - convex.db (database)")
	fmt.Println("  - storage/ (file storage)")
	fmt.Println("  - manifest.json")
	fmt.Println("  - credentials.json")

	return nil
}

func runInfo(cmd *cobra.Command, args []string) error {
	output := InfoOutput{
		IsSelfHost: isSelfHostMode,
	}

	if !isSelfHostMode {
		if flagJSON {
			return printJSONOutput(output)
		}
		fmt.Println("Convex Self-Host Bundle")
		fmt.Println("=======================")
		fmt.Println()
		fmt.Println("Status: Not a self-host executable")
		fmt.Println()
		fmt.Println("This executable does not contain an embedded bundle.")
		fmt.Println("Use --bundle flag with install/upgrade commands to specify a bundle directory.")
		return nil
	}

	// Read header
	header, err := selfhost.ReadHeaderFromExecutable("")
	if err != nil {
		return fmt.Errorf("failed to read bundle header: %w", err)
	}

	output.OpsVersion = header.OpsVersion
	output.BundleName = header.Manifest.Name
	output.BundleVersion = header.Manifest.Version
	output.Platform = header.Manifest.Platform
	output.CreatedAt = header.CreatedAt
	output.Apps = header.Manifest.Apps
	output.BundleSize = header.BundleSize
	output.Compression = header.Compression
	output.BundleChecksum = header.BundleChecksum

	if flagJSON {
		return printJSONOutput(output)
	}

	// Human-readable output
	fmt.Println("Convex Self-Host Bundle")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Printf("Ops Version:    %s\n", valueOrDefault(header.OpsVersion, "unknown"))
	fmt.Printf("Bundle Name:    %s\n", header.Manifest.Name)
	fmt.Printf("Bundle Version: %s\n", header.Manifest.Version)
	fmt.Printf("Platform:       %s\n", header.Manifest.Platform)
	fmt.Printf("Created:        %s\n", header.CreatedAt)
	fmt.Println()

	if len(header.Manifest.Apps) > 0 {
		fmt.Println("Bundled Apps:")
		for _, app := range header.Manifest.Apps {
			fmt.Printf("  - %s\n", app)
		}
		fmt.Println()
	}

	fmt.Printf("Bundle Size:    %s (uncompressed)\n", formatBytes(header.BundleSize))
	fmt.Printf("Compression:    %s\n", header.Compression)
	fmt.Printf("Checksum:       %s\n", header.BundleChecksum)

	return nil
}

func runVerify(cmd *cobra.Command, args []string) error {
	if !isSelfHostMode {
		if flagJSON {
			return printJSONOutput(VerifyOutput{Valid: false})
		}
		return fmt.Errorf("this executable does not contain an embedded bundle")
	}

	printInfo("Verifying bundle integrity...")

	result, err := selfhost.Verify("")
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	output := VerifyOutput{
		Valid:            result.Valid,
		ExpectedChecksum: result.ExpectedChecksum,
		ActualChecksum:   result.ActualChecksum,
	}

	if flagJSON {
		return printJSONOutput(output)
	}

	if result.Valid {
		printSuccess("Bundle integrity verified")
		fmt.Printf("  Checksum: %s (matched)\n", result.ExpectedChecksum)
		return nil
	}

	printError("Bundle integrity verification FAILED!")
	fmt.Printf("  Expected: %s\n", result.ExpectedChecksum)
	fmt.Printf("  Actual:   %s\n", result.ActualChecksum)
	fmt.Println()
	fmt.Println("The bundle may be corrupted. Please re-download the executable.")
	os.Exit(selfhost.ExitVerificationFailed)
	return nil
}

// Helper functions

func printJSONOutput(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func valueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
