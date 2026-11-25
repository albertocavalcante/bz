package cache

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/depgraph"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
)

// Match the maximum MODULE.bazel response accepted by the HTTP registry.
const maxCachedModuleBazelSize = 50 << 20

var (
	verifyJSON     bool
	verifyCacheDir string
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify cached MODULE.bazel dependency closure",
	Long: `Checks that cached MODULE.bazel files are readable and parseable
across exact declared versions, including development dependencies, from the
current MODULE.bazel file.
Bazel resolution and source archive availability require separate checks.

Examples:
  bz cache verify
  bz cache verify --json
  bz cache verify --cache-dir=/path/to/cache`,
	RunE:          runVerify,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func configureVerifyCmd() {
	verifyCmd.Flags().BoolVar(&verifyJSON, "json", false, "Output as JSON")
	verifyCmd.Flags().StringVar(&verifyCacheDir, "cache-dir", "", "Cache directory (default: ~/.cache/bz)")
	Cmd.AddCommand(verifyCmd)
}

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

	cacheDir, err := resolveCacheDir(verifyCacheDir)
	if err != nil {
		return err
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
		fmt.Fprintln(out, "\nCache is COMPLETE for declared MODULE.bazel closure.")
		return nil
	}

	if !verifyJSON {
		fmt.Fprintln(out, "Verifying cache for MODULE.bazel dependencies...")
		fmt.Fprintln(out)
	}

	statuses := verifyCachedClosure(cacheDir, depgraph.FromModuleFileDeps(f))

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
			fmt.Fprintln(out, cli.Success("Cache is COMPLETE for declared MODULE.bazel closure."))
		} else {
			fmt.Fprintf(out, cli.Error("Cache is INCOMPLETE for declared MODULE.bazel closure.")+" Run '%s' to fix.\n", "bz cache download")
		}
	}

	if !result.Complete {
		return fmt.Errorf("cache incomplete: %d module(s) missing", result.Missing)
	}

	return nil
}

// verifyCachedClosure walks the same declared dependency closure that cache
// download follows. A module is cached only when its MODULE.bazel can be read
// safely and parsed, because otherwise its children cannot be determined.
func verifyCachedClosure(cacheDir string, initial []depgraph.ModuleRef) []moduleStatus {
	seen := make(map[string]bool)
	queue := append([]depgraph.ModuleRef(nil), initial...)
	statuses := make([]moduleStatus, 0, len(initial))

	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		if seen[ref.Key()] {
			continue
		}
		seen[ref.Key()] = true

		content, err := readCachedModuleBazel(cacheDir, ref)
		if err != nil {
			statuses = append(statuses, moduleStatus{name: ref.Name, version: ref.Version})
			continue
		}
		parsed, err := module.LoadContent(ref.Name, content)
		if err != nil {
			statuses = append(statuses, moduleStatus{name: ref.Name, version: ref.Version})
			continue
		}

		statuses = append(statuses, moduleStatus{name: ref.Name, version: ref.Version, cached: true})
		queue = append(queue, depgraph.FromModuleFileDeps(parsed)...)
	}

	return statuses
}

// readCachedModuleBazel reads one cache entry through an os.Root so the path
// stays beneath cacheDir. The nonblocking open avoids waiting on a FIFO. The
// post-open stat and bounded read reject special or oversized files.
func readCachedModuleBazel(cacheDir string, ref depgraph.ModuleRef) ([]byte, error) {
	if err := coord.Validate(ref.Name, ref.Version); err != nil {
		return nil, err
	}

	// Keep cacheDir and modules as traversed directory components. Opening a
	// FIFO directly can block before it can be checked with fstat.
	base, err := os.OpenRoot(cacheDir + string(os.PathSeparator) + ".")
	if err != nil {
		return nil, err
	}
	defer base.Close()
	root, err := base.OpenRoot(registry.ModulesDir + "/.")
	if err != nil {
		return nil, err
	}
	defer root.Close()

	path := filepath.Join(ref.Name, ref.Version, "MODULE.bazel")
	file, err := root.OpenFile(path, verificationOpenFlags(), 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("cached MODULE.bazel is not a regular file")
	}
	if info.Size() > maxCachedModuleBazelSize {
		return nil, fmt.Errorf("cached MODULE.bazel exceeds %d-byte limit", maxCachedModuleBazelSize)
	}

	content, err := io.ReadAll(io.LimitReader(file, maxCachedModuleBazelSize+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxCachedModuleBazelSize {
		return nil, fmt.Errorf("cached MODULE.bazel exceeds %d-byte limit", maxCachedModuleBazelSize)
	}
	return content, nil
}

func printVerifyText(w io.Writer, statuses []moduleStatus) {
	for _, s := range statuses {
		label := (moduleVersion{name: s.name, version: s.version}).label()
		if s.cached {
			fmt.Fprintf(w, "  %s %s\n", cli.SuccessIcon(), label)
		} else {
			fmt.Fprintf(w, "  %s %s %s\n", cli.ErrorIcon(), label, cli.Muted("(missing)"))
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

	return cmdutil.WriteJSON(w, result)
}
