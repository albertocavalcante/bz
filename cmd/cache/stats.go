package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	statsJSON     bool
	statsCacheDir string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	Long: `Shows statistics about the local module cache.

Displays information including:
  - Cache directory location
  - Total number of cached modules
  - Total cache size
  - Last updated timestamp

Examples:
  bz cache stats
  bz cache stats --json
  bz cache stats --cache-dir=/path/to/cache`,
	RunE:          runStats,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func configureStatsCmd() {
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output as JSON")
	statsCmd.Flags().StringVar(&statsCacheDir, "cache-dir", "", "Cache directory (default: ~/.cache/bz)")
	Cmd.AddCommand(statsCmd)
}

// statsResult holds the result of a stats operation for JSON output.
type statsResult struct {
	CacheDir       string    `json:"cache_dir"`
	TotalModules   int       `json:"total_modules"`
	TotalVersions  int       `json:"total_versions"`
	TotalSizeBytes int64     `json:"total_size_bytes"`
	TotalSize      string    `json:"total_size"`
	LastUpdated    time.Time `json:"last_updated"`
}

func runStats(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	cacheDir, err := resolveCacheDir(statsCacheDir)
	if err != nil {
		return err
	}

	// Calculate statistics
	stats := calculateCacheStats(cacheDir)
	stats.CacheDir = cacheDir

	if statsJSON {
		return printStatsJSON(out, stats)
	}
	return printStatsText(out, stats)
}

func calculateCacheStats(cacheDir string) statsResult {
	stats := statsResult{}

	modulesDir := filepath.Join(cacheDir, "modules")

	// Check if modules directory exists
	if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
		return stats
	}

	var lastModTime time.Time

	// Walk the modules directory
	err := filepath.WalkDir(modulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible entries during stats collection
		}

		// Count modules (directories directly under modules/)
		rel, _ := filepath.Rel(modulesDir, path)
		if d.IsDir() && rel != "." && !containsPathSeparator(rel) {
			stats.TotalModules++
		}

		// Count versions (directories two levels deep that contain MODULE.bazel)
		if d.IsDir() && filepath.Base(path) != "modules" {
			moduleBazelPath := filepath.Join(path, "MODULE.bazel")
			if _, err := os.Stat(moduleBazelPath); err == nil {
				stats.TotalVersions++
			}
		}

		// Calculate size
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				stats.TotalSizeBytes += info.Size()
				if info.ModTime().After(lastModTime) {
					lastModTime = info.ModTime()
				}
			}
		}

		return nil
	})

	if err == nil && !lastModTime.IsZero() {
		stats.LastUpdated = lastModTime
	}

	stats.TotalSize = formatSize(stats.TotalSizeBytes)

	return stats
}

// containsPathSeparator checks if a path contains a path separator.
func containsPathSeparator(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] == filepath.Separator {
			return true
		}
	}
	return false
}

// formatSize formats a byte count as a human-readable string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

func printStatsText(w io.Writer, stats statsResult) error {
	fmt.Fprintf(w, "Cache directory: %s\n", stats.CacheDir)
	fmt.Fprintf(w, "Total modules:   %d\n", stats.TotalModules)
	fmt.Fprintf(w, "Total versions:  %d\n", stats.TotalVersions)
	fmt.Fprintf(w, "Total size:      %s\n", stats.TotalSize)
	if !stats.LastUpdated.IsZero() {
		fmt.Fprintf(w, "Last updated:    %s\n", stats.LastUpdated.Format("2006-01-02 15:04:05"))
	}
	return nil
}

func printStatsJSON(w io.Writer, stats statsResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(stats)
}
