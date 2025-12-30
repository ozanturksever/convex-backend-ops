package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	flagYes   bool
	flagQuiet bool
	flagJSON  bool

	// Build info (set via ldflags)
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "convex-backend-ops",
	Short: "Operations tool for Convex backend",
	Long: `convex-backend-ops is a single-binary operations tool for deploying 
and managing Convex backend with pre-deployed apps.

It can be deployed on air-gapped or restricted network environments 
without needing Node.js.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&flagYes, "yes", "y", false, "Skip all confirmation prompts")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output results in JSON format")
}

// Helper functions for output

func printInfo(format string, args ...interface{}) {
	if !flagQuiet {
		fmt.Printf(format+"\n", args...)
	}
}

func printError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

func printSuccess(format string, args ...interface{}) {
	if !flagQuiet {
		fmt.Printf("✓ "+format+"\n", args...)
	}
}
