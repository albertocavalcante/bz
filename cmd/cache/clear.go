package cache

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
)

var (
	clearJSON     bool
	clearForce    bool
	clearCacheDir string
)

var clearCmd = &cobra.Command{
	Use:   "clear [modules...]",
	Short: "Clear the local module cache",
	Long: `Clears the local module cache.

If no modules are specified, clears the entire cache.
If modules are specified, only those modules are removed.

Use --force to skip confirmation prompts.

Examples:
  bz cache clear                  # Clear entire cache (with confirmation)
  bz cache clear --force          # Clear without confirmation
  bz cache clear rules_go         # Clear only rules_go from cache
  bz cache clear --json           # Output as JSON`,
	RunE:          runClear,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func configureClearCmd() {
	clearCmd.Flags().BoolVar(&clearJSON, "json", false, "Output as JSON")
	clearCmd.Flags().BoolVarP(&clearForce, "force", "f", false, "Skip confirmation prompt")
	clearCmd.Flags().StringVar(&clearCacheDir, "cache-dir", "", "Cache directory (default: ~/.cache/bz)")
	Cmd.AddCommand(clearCmd)
}

// clearResult holds the result of a clear operation for JSON output.
type clearResult struct {
	ModulesCleared int      `json:"modules_cleared"`
	BytesCleared   int64    `json:"bytes_cleared"`
	SizeCleared    string   `json:"size_cleared"`
	Modules        []string `json:"modules,omitempty"`
}

func runClear(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	in := cmd.InOrStdin()

	cacheDir, err := resolveCacheDir(clearCacheDir)
	if err != nil {
		return err
	}

	modulesDir := filepath.Join(cacheDir, "modules")

	// Check if cache exists
	if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
		if clearJSON {
			return printClearJSON(out, clearResult{})
		}
		fmt.Fprintln(out, "Cache already empty or does not exist.")
		fmt.Fprintln(out, cli.Success("Cache cleared."))
		return nil
	}

	// Determine what to clear
	var modulesToClear []string
	if len(args) > 0 {
		// Clear specific modules
		modulesToClear = args
	} else {
		// Clear all modules
		entries, err := os.ReadDir(modulesDir)
		if err != nil {
			return fmt.Errorf("failed to read cache directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				modulesToClear = append(modulesToClear, entry.Name())
			}
		}
	}

	if len(modulesToClear) == 0 {
		if clearJSON {
			return printClearJSON(out, clearResult{})
		}
		fmt.Fprintln(out, "Cache already empty.")
		fmt.Fprintln(out, cli.Success("Cache cleared."))
		return nil
	}

	// Calculate size before clearing
	var totalSize int64
	for _, moduleName := range modulesToClear {
		moduleDir := filepath.Join(modulesDir, moduleName)
		size := calculateDirSize(moduleDir)
		totalSize += size
	}

	// Confirm if not forced
	if !clearForce && !clearJSON {
		fmt.Fprintf(out, "This will clear %d module(s) (%s) from the cache.\n", len(modulesToClear), formatSize(totalSize))
		fmt.Fprint(out, "Continue? [y/N] ")

		reader := bufio.NewReader(in)
		response, err := reader.ReadString('\n')
		if err != nil {
			return nil //nolint:nilerr // abort on read error (e.g. stdin closed); treat as user declining
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(out, "Aborted.")
			return nil
		}
	}

	// Clear modules
	result := clearResult{
		BytesCleared: totalSize,
		SizeCleared:  formatSize(totalSize),
	}

	for _, moduleName := range modulesToClear {
		moduleDir := filepath.Join(modulesDir, moduleName)
		if err := os.RemoveAll(moduleDir); err != nil {
			if !clearJSON {
				fmt.Fprintf(out, "  %s Failed to clear %s: %v\n", cli.ErrorIcon(), moduleName, err)
			}
			continue
		}
		result.ModulesCleared++
		result.Modules = append(result.Modules, moduleName)
	}

	if clearJSON {
		return printClearJSON(out, result)
	}

	if result.ModulesCleared == 1 {
		fmt.Fprintf(out, "Cleared 1 module (%s) from cache.\n", result.SizeCleared)
	} else {
		fmt.Fprintf(out, "Cleared %d modules (%s) from cache.\n", result.ModulesCleared, result.SizeCleared)
	}
	fmt.Fprintln(out, cli.Success("Cache cleared."))

	return nil
}

// calculateDirSize calculates the total size of a directory.
func calculateDirSize(dir string) int64 {
	var size int64
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible entries during size calculation
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}

func printClearJSON(w io.Writer, result clearResult) error {
	if result.Modules == nil {
		result.Modules = []string{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
