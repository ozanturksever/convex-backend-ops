package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// BackupInfo represents backup information for display
type BackupInfo struct {
	Version   string `json:"version"`
	Created   string `json:"created"`
	Size      int64  `json:"size"`
	SizeHuman string `json:"sizeHuman"`
	Reason    string `json:"reason"`
	Path      string `json:"path"`
}

// ListBackupsOutput represents JSON output for list-backups command
type ListBackupsOutput struct {
	Backups    []BackupInfo `json:"backups"`
	TotalCount int          `json:"totalCount"`
	TotalSize  int64        `json:"totalSize"`
}

var listBackupsCmd = &cobra.Command{
	Use:   "list-backups",
	Short: "List all available backups",
	Long:  `List all available backups with version, creation time, and size.`,
	RunE:  runListBackups,
}

func init() {
	rootCmd.AddCommand(listBackupsCmd)
}

func runListBackups(cmd *cobra.Command, args []string) error {
	backupsDir := "/var/lib/convex/backups"

	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			if flagJSON {
				return printJSON(ListBackupsOutput{Backups: []BackupInfo{}})
			}
			fmt.Println("No backups found.")
			return nil
		}
		return fmt.Errorf("failed to read backups directory: %w", err)
	}

	var backups []BackupInfo
	var totalSize int64

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		backupPath := filepath.Join(backupsDir, entry.Name())
		metaPath := filepath.Join(backupPath, "meta.json")

		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var meta BackupMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		// Calculate directory size
		size := getDirSize(backupPath)
		totalSize += size

		backups = append(backups, BackupInfo{
			Version:   meta.Version,
			Created:   meta.Timestamp,
			Size:      size,
			SizeHuman: humanizeBytes(size),
			Reason:    meta.Reason,
			Path:      backupPath,
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, backups[i].Created)
		tj, _ := time.Parse(time.RFC3339, backups[j].Created)
		return ti.After(tj)
	})

	output := ListBackupsOutput{
		Backups:    backups,
		TotalCount: len(backups),
		TotalSize:  totalSize,
	}

	if flagJSON {
		return printJSON(output)
	}

	// Human-readable output
	if len(backups) == 0 {
		fmt.Println("No backups found.")
		return nil
	}

	fmt.Println("Available Backups")
	fmt.Println("=================")
	fmt.Println()
	fmt.Printf("%-10s %-20s %-10s %s\n", "VERSION", "CREATED", "SIZE", "REASON")

	for _, b := range backups {
		created := b.Created
		if t, err := time.Parse(time.RFC3339, b.Created); err == nil {
			created = t.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("v%-9s %-20s %-10s %s\n", b.Version, created, b.SizeHuman, b.Reason)
	}

	fmt.Println()
	fmt.Printf("Total: %d backups (%s)\n", len(backups), humanizeBytes(totalSize))

	return nil
}

func getDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func humanizeBytes(bytes int64) string {
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
