package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// Credentials represents the credentials.json from bundle
type Credentials struct {
	AdminKey       string `json:"adminKey"`
	InstanceSecret string `json:"instanceSecret"`
}

var installBundlePath string

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Convex backend from a bundle",
	Long:  `Install Convex backend from a bundle created by convex-bundler.`,
	RunE:  runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringVarP(&installBundlePath, "bundle", "b", "", "Path to the bundle directory (uses embedded bundle if not specified)")
	// Note: --bundle is NOT marked as required because self-host executables have embedded bundles
}

func runInstall(cmd *cobra.Command, args []string) error {
	// Pre-flight checks
	if err := checkRoot(); err != nil {
		return err
	}

	if err := checkSystemd(); err != nil {
		return err
	}

	if err := checkNotInstalled(); err != nil {
		return err
	}

	// Determine bundle path - either from flag or from embedded bundle
	bundlePath := installBundlePath
	var cleanupFunc func()

	if bundlePath == "" {
		// No --bundle flag provided, check if we're in self-host mode
		if !IsSelfHostMode() {
			return fmt.Errorf("--bundle flag is required (or run a self-host executable with embedded bundle)")
		}

		printInfo("Using embedded bundle from self-host executable...")

		// Extract embedded bundle to temp directory
		printInfo("Extracting embedded bundle...")
		var err error
		bundlePath, cleanupFunc, err = GetEmbeddedBundlePath()
		if err != nil {
			return fmt.Errorf("failed to extract embedded bundle: %w", err)
		}

		printInfo("Bundle extracted to temporary directory")
	}

	// Ensure cleanup runs if we extracted an embedded bundle
	if cleanupFunc != nil {
		defer cleanupFunc()
	}

	if err := validateBundle(bundlePath); err != nil {
		return fmt.Errorf("invalid bundle: %w", err)
	}

	printInfo("Installing Convex backend from bundle: %s", bundlePath)

	// Create directory structure
	printInfo("Creating directories...")
	if err := createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Copy bundle assets
	printInfo("Copying bundle assets...")
	if err := copyBundleAssets(bundlePath); err != nil {
		return fmt.Errorf("failed to copy bundle assets: %w", err)
	}

	// Extract credentials
	printInfo("Extracting credentials...")
	creds, err := extractCredentials(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to extract credentials: %w", err)
	}

	if err := writeCredentials(creds); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	// Create environment config
	printInfo("Creating environment config...")
	if err := createEnvConfig(); err != nil {
		return fmt.Errorf("failed to create environment config: %w", err)
	}

	// Install systemd service
	printInfo("Installing systemd service...")
	if err := installSystemdService(); err != nil {
		return fmt.Errorf("failed to install systemd service: %w", err)
	}

	// Copy manifest
	printInfo("Copying manifest...")
	if err := copyFile(
		filepath.Join(bundlePath, "manifest.json"),
		"/var/lib/convex/manifest.json",
	); err != nil {
		return fmt.Errorf("failed to copy manifest: %w", err)
	}

	// Start service
	printInfo("Starting service...")
	if err := startService(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Health check
	printInfo("Waiting for backend to be ready...")
	if err := waitForHealth("http://localhost:3210", 30*time.Second); err != nil {
		// Show logs on failure
		showServiceLogs()
		return fmt.Errorf("health check failed: %w", err)
	}

	// Read manifest for output
	manifest, _ := readManifest("/var/lib/convex/manifest.json")

	printSuccess("Convex backend installed successfully!")
	fmt.Println()
	fmt.Println("Backend URL:  http://localhost:3210")
	fmt.Printf("Admin Key:    %s\n", creds.AdminKey)
	fmt.Println()

	if manifest != nil && len(manifest.Apps) > 0 {
		fmt.Println("Bundled Apps:")
		for _, app := range manifest.Apps {
			fmt.Printf("  - %s\n", app)
		}
		fmt.Println()
	}

	fmt.Println("Service commands:")
	fmt.Println("  systemctl status convex-backend")
	fmt.Println("  journalctl -u convex-backend -f")

	return nil
}

func checkRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root (use sudo)")
	}
	return nil
}

func checkSystemd() error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemd is required but not found")
	}
	return nil
}

func checkNotInstalled() error {
	if _, err := os.Stat("/var/lib/convex/manifest.json"); err == nil {
		return fmt.Errorf("Convex backend is already installed. Use 'upgrade' to update")
	}
	return nil
}

func validateBundle(bundlePath string) error {
	requiredFiles := []string{"backend", "convex.db", "manifest.json", "credentials.json"}
	for _, f := range requiredFiles {
		path := filepath.Join(bundlePath, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("missing required file: %s", f)
		}
	}
	return nil
}

func createDirectories() error {
	dirs := []string{
		"/var/lib/convex",
		"/var/lib/convex/data",
		"/var/lib/convex/data/storage",
		"/var/lib/convex/backups",
		"/etc/convex",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}
	return nil
}

func copyBundleAssets(bundlePath string) error {
	// Copy backend binary
	if err := copyFile(
		filepath.Join(bundlePath, "backend"),
		"/usr/local/bin/convex-backend",
	); err != nil {
		return fmt.Errorf("failed to copy backend binary: %w", err)
	}

	// Make binary executable
	if err := os.Chmod("/usr/local/bin/convex-backend", 0755); err != nil {
		return fmt.Errorf("failed to set binary permissions: %w", err)
	}

	// Copy database
	if err := copyFile(
		filepath.Join(bundlePath, "convex.db"),
		"/var/lib/convex/data/convex.db",
	); err != nil {
		return fmt.Errorf("failed to copy database: %w", err)
	}

	// Copy storage directory
	storageSrc := filepath.Join(bundlePath, "storage")
	if _, err := os.Stat(storageSrc); err == nil {
		if err := copyDir(storageSrc, "/var/lib/convex/data/storage"); err != nil {
			return fmt.Errorf("failed to copy storage: %w", err)
		}
	}

	return nil
}

func extractCredentials(bundlePath string) (*Credentials, error) {
	data, err := os.ReadFile(filepath.Join(bundlePath, "credentials.json"))
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

func writeCredentials(creds *Credentials) error {
	// Write admin key
	if err := os.WriteFile("/etc/convex/admin.key", []byte(creds.AdminKey), 0600); err != nil {
		return fmt.Errorf("failed to write admin key: %w", err)
	}

	// Write instance secret
	if err := os.WriteFile("/etc/convex/instance.secret", []byte(creds.InstanceSecret), 0600); err != nil {
		return fmt.Errorf("failed to write instance secret: %w", err)
	}

	return nil
}

func createEnvConfig() error {
	envContent := `CONVEX_SITE_URL=http://localhost:3210
CONVEX_LOCAL_STORAGE=/var/lib/convex/data
CONVEX_ADMIN_KEY_FILE=/etc/convex/admin.key
CONVEX_INSTANCE_SECRET_FILE=/etc/convex/instance.secret
`
	return os.WriteFile("/etc/convex/convex.env", []byte(envContent), 0644)
}

func installSystemdService() error {
	serviceContent := `[Unit]
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
`
	if err := os.WriteFile("/etc/systemd/system/convex-backend.service", []byte(serviceContent), 0644); err != nil {
		return err
	}

	// Reload systemd
	cmd := exec.Command("systemctl", "daemon-reload")
	return cmd.Run()
}

func startService() error {
	// Enable service
	if err := exec.Command("systemctl", "enable", "convex-backend").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	// Start service
	if err := exec.Command("systemctl", "start", "convex-backend").Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func waitForHealth(url string, timeout time.Duration) error {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/version")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("backend did not become healthy within %v", timeout)
}

func showServiceLogs() {
	printError("Recent service logs:")
	cmd := exec.Command("journalctl", "-u", "convex-backend", "-n", "20", "--no-pager")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func readManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
