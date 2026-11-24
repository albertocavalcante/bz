package cache

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/coord"
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
	if err := cli.CheckCommandAllowed("clear"); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	in := cmd.InOrStdin()

	if err := validateClearArgs(args); err != nil {
		return err
	}

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

	totalSize, err := targetsSize(modulesDir, modulesToClear)
	if err != nil {
		return err
	}

	// --json cannot prompt, so it never implies consent.
	if !clearForce && clearJSON {
		return fmt.Errorf("refusing to clear %d module(s) without confirmation: pass --force with --json", len(modulesToClear))
	}

	// Confirm if not forced
	if !clearForce && !confirmClear(out, in, len(modulesToClear), totalSize) {
		return nil
	}

	// Clear modules
	result := clearResult{
		BytesCleared: totalSize,
		SizeCleared:  formatSize(totalSize),
	}

	// Deletes go through os.Root so that nothing outside modulesDir can be
	// removed, even through a symlinked path component.
	root, err := os.OpenRoot(modulesDir)
	if err != nil {
		return fmt.Errorf("open cache directory: %w", err)
	}
	defer root.Close()

	for _, moduleName := range modulesToClear {
		if err := root.RemoveAll(moduleName); err != nil {
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

// validateClearArgs rejects module arguments that are not valid module
// names. They become path segments under the cache, so this runs before any
// filesystem access.
func validateClearArgs(args []string) error {
	for _, arg := range args {
		if err := coord.ValidateModuleName(arg); err != nil {
			return fmt.Errorf("refusing to clear %q: %w", arg, err)
		}
	}
	return nil
}

// targetsSize returns the total size of the named module directories,
// refusing any name that does not resolve strictly inside modulesDir.
func targetsSize(modulesDir string, names []string) (int64, error) {
	var total int64
	for _, name := range names {
		moduleDir, err := coord.Join(modulesDir, name)
		if err != nil {
			return 0, fmt.Errorf("refusing to clear %q: %w", name, err)
		}
		total += calculateDirSize(moduleDir)
	}
	return total, nil
}

// confirmClear prompts for confirmation. A read error (for example, stdin
// closed) is treated as the user declining.
func confirmClear(out io.Writer, in io.Reader, count int, size int64) bool {
	fmt.Fprintf(out, "This will clear %d module(s) (%s) from the cache.\n", count, formatSize(size))
	fmt.Fprint(out, "Continue? [y/N] ")

	response, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Fprintln(out, "Aborted.")
		return false
	}
	return true
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
	return cmdutil.WriteJSON(w, result)
}
