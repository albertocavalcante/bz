package mod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
)

var (
	statsJSON bool
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show dependency statistics",
	Long: `Display statistics about module dependencies.

Shows counts of direct and transitive dependencies, total modules,
maximum dependency depth, and dev dependencies.

Examples:
  bz mod stats
  bz mod stats --json
  bz mod stats --registry=/path/to/registry`,
	RunE: runStats,
}

func init() {
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output as JSON")
	Cmd.AddCommand(statsCmd)
}

// Stats holds dependency statistics.
type Stats struct {
	DirectDeps     int `json:"direct_dependencies"`
	TransitiveDeps int `json:"transitive_dependencies"`
	TotalModules   int `json:"total_modules"`
	MaxDepth       int `json:"max_depth"`
	DevDeps        int `json:"dev_dependencies"`
}

// moduleKey uniquely identifies a module by name@version.
type moduleKey struct {
	name    string
	version string
}

func runStats(cmd *cobra.Command, args []string) error {
	// Check if command is disabled
	if err := cli.CheckCommandAllowed("stats"); err != nil {
		return err
	}

	f, err := module.FindAndLoad()
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Create network-aware registry
	reg, err := createNetworkAwareRegistry()
	if err != nil {
		return err
	}

	var stats *Stats
	err = cli.WithSpinner("Calculating statistics...", func() error {
		var calcErr error
		stats, calcErr = calculateStats(ctx, reg, f)
		return calcErr
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if statsJSON {
		return printStatsJSON(out, stats)
	}

	return printStatsTable(out, stats)
}

func calculateStats(ctx context.Context, reg registry.Registry, f *module.File) (*Stats, error) {
	stats := &Stats{}

	// Count direct dependencies
	stats.DirectDeps = len(f.Deps)

	// Count dev dependencies
	for _, dep := range f.Deps {
		if dep.DevDependency {
			stats.DevDeps++
		}
	}

	// If no dependencies, return early
	if stats.DirectDeps == 0 {
		return stats, nil
	}

	// Track all modules we've seen (to avoid counting duplicates)
	allModules := make(map[moduleKey]bool)

	// Add direct deps to all modules set
	for _, dep := range f.Deps {
		key := moduleKey{name: dep.Name.String(), version: dep.Version.String()}
		allModules[key] = true
	}

	// Calculate max depth and find all transitive deps
	maxDepth := 0
	visited := make(map[moduleKey]bool)

	for _, dep := range f.Deps {
		key := moduleKey{name: dep.Name.String(), version: dep.Version.String()}
		depth := resolveTransitiveDeps(ctx, reg, key, allModules, visited, 1)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	stats.MaxDepth = maxDepth
	stats.TotalModules = len(allModules)

	// Transitive deps = total - direct
	stats.TransitiveDeps = stats.TotalModules - stats.DirectDeps

	return stats, nil
}

// resolveTransitiveDeps recursively resolves dependencies and returns the max depth.
func resolveTransitiveDeps(
	ctx context.Context,
	reg registry.Registry,
	key moduleKey,
	allModules map[moduleKey]bool,
	visited map[moduleKey]bool,
	currentDepth int,
) int {
	// Avoid infinite loops for circular dependencies
	if visited[key] {
		return currentDepth
	}
	visited[key] = true

	// Fetch the module's MODULE.bazel to get its dependencies
	content, err := reg.GetModuleBazel(ctx, key.name, key.version)
	if err != nil {
		// If we can't fetch the module, just return current depth
		return currentDepth
	}

	// Parse the MODULE.bazel
	modFile, err := module.LoadContent(key.name, content)
	if err != nil {
		return currentDepth
	}

	// No dependencies means we're at a leaf
	if len(modFile.Deps) == 0 {
		return currentDepth
	}

	maxDepth := currentDepth

	for _, dep := range modFile.Deps {
		depKey := moduleKey{name: dep.Name.String(), version: dep.Version.String()}

		// Track this module
		allModules[depKey] = true

		// Recursively resolve
		depth := resolveTransitiveDeps(ctx, reg, depKey, allModules, visited, currentDepth+1)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
}

func printStatsTable(w io.Writer, stats *Stats) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw)
	fmt.Fprintln(tw, "Dependency Statistics:")
	fmt.Fprintf(tw, "  Direct dependencies:\t%d\n", stats.DirectDeps)
	fmt.Fprintf(tw, "  Transitive dependencies:\t%d\n", stats.TransitiveDeps)
	fmt.Fprintf(tw, "  Total modules:\t%d\n", stats.TotalModules)
	fmt.Fprintf(tw, "  Max depth:\t%d\n", stats.MaxDepth)
	fmt.Fprintf(tw, "  Dev dependencies:\t%d\n", stats.DevDeps)

	return tw.Flush()
}

func printStatsJSON(w io.Writer, stats *Stats) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(stats)
}
