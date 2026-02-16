package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/module"
)

var (
	verifyJSON     bool
	verifyCacheDir string
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify cache is complete for offline use",
	Long: `Verifies that the local cache contains all dependencies required
for offline use based on the current MODULE.bazel file.

This command checks that all direct dependencies (and optionally transitive
dependencies) are present in the local cache.

Examples:
  bz cache verify
  bz cache verify --json
  bz cache verify --cache-dir=/path/to/cache`,
	RunE:          runVerify,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var _ = onLoad(func() {
	verifyCmd.Flags().BoolVar(&verifyJSON, "json", false, "Output as JSON")
	verifyCmd.Flags().StringVar(&verifyCacheDir, "cache-dir", "", "Cache directory (default: ~/.cache/bz)")
	Cmd.AddCommand(verifyCmd)
})

// verifyResult holds the result of a verify operation for JSON output.
type verifyResult struct {
	Complete       bool     `json:"complete"`
	Cached         int      `json:"cached"`
	Missing        int      `json:"missing"`
	CachedModules  []string `json:"cached_modules"`
	MissingModules []string `json:"missing_modules"`
}

// moduleStatus represents the cache status of a single module.
type moduleStatus struct {
	name    string
	version string
	cached  bool
}

func runVerify(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	// Determine cache directory
	cacheDir := verifyCacheDir
	if cacheDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = filepath.Join(homeDir, ".cache", "bz")
	}

	// Load MODULE.bazel
	f, err := module.FindAndLoad()
	if err != nil {
		return fmt.Errorf("failed to load MODULE.bazel: %w", err)
	}

	if len(f.Deps) == 0 {
		if verifyJSON {
			return printVerifyJSON(out, verifyResult{Complete: true})
		}
		fmt.Fprintln(out, "No dependencies found in MODULE.bazel")
		fmt.Fprintln(out, "\nCache is COMPLETE for offline use.")
		return nil
	}

	if !verifyJSON {
		fmt.Fprintln(out, "Verifying cache for MODULE.bazel dependencies...")
		fmt.Fprintln(out)
	}

	// Check each dependency
	var statuses []moduleStatus
	for _, dep := range f.Deps {
		name := dep.Name.String()
		version := dep.Version.String()

		cached := isModuleCached(cacheDir, name, version)
		statuses = append(statuses, moduleStatus{
			name:    name,
			version: version,
			cached:  cached,
		})
	}

	// Build result
	result := verifyResult{}
	for _, s := range statuses {
		modVer := fmt.Sprintf("%s@%s", s.name, s.version)
		if s.cached {
			result.Cached++
			result.CachedModules = append(result.CachedModules, modVer)
		} else {
			result.Missing++
			result.MissingModules = append(result.MissingModules, modVer)
		}
	}
	result.Complete = result.Missing == 0

	if verifyJSON {
		if err := printVerifyJSON(out, result); err != nil {
			return err
		}
	} else {
		printVerifyText(out, statuses)

		fmt.Fprintln(out)
		if result.Complete {
			fmt.Fprintln(out, cli.Success("Cache is COMPLETE for offline use."))
		} else {
			fmt.Fprintf(out, cli.Error("Cache is INCOMPLETE.")+" Run '%s' to fix.\n", "bz cache download")
		}
	}

	if !result.Complete {
		return fmt.Errorf("cache incomplete: %d module(s) missing", result.Missing)
	}

	return nil
}

// isModuleCached checks if a specific module version exists in the cache.
func isModuleCached(cacheDir, name, version string) bool {
	moduleBazelPath := filepath.Join(cacheDir, "modules", name, version, "MODULE.bazel")
	_, err := os.Stat(moduleBazelPath)
	return err == nil
}

func printVerifyText(w io.Writer, statuses []moduleStatus) {
	for _, s := range statuses {
		if s.cached {
			fmt.Fprintf(w, "  %s %s@%s\n", cli.SuccessIcon(), s.name, s.version)
		} else {
			fmt.Fprintf(w, "  %s %s@%s %s\n", cli.ErrorIcon(), s.name, s.version, cli.Muted("(missing)"))
		}
	}
}

func printVerifyJSON(w io.Writer, result verifyResult) error {
	// Ensure slices are not nil for cleaner JSON output
	if result.CachedModules == nil {
		result.CachedModules = []string{}
	}
	if result.MissingModules == nil {
		result.MissingModules = []string{}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
