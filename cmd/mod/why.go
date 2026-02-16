package mod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
)

var (
	whyJSON bool
	whyAll  bool
)

var whyCmd = &cobra.Command{
	Use:   "why <module>",
	Short: "Explain why a module is a dependency",
	Long: `Show the dependency path(s) that bring a module into your project.

This command traces through your dependency graph to show why a particular
module is included, whether as a direct or transitive dependency.

Examples:
  bz mod why protobuf
  bz mod why protobuf --all
  bz mod why protobuf --json`,
	RunE: runWhy,
}

func configureWhyCmd() {
	whyCmd.Flags().BoolVar(&whyJSON, "json", false, "Output as JSON")
	whyCmd.Flags().BoolVar(&whyAll, "all", false, "Show all paths (default: shortest paths only)")
	Cmd.AddCommand(whyCmd)
}

// WhyResult represents the result of finding dependency paths.
type WhyResult struct {
	Module string     `json:"module"`
	Found  bool       `json:"found"`
	Paths  [][]string `json:"paths"`
}

// depNode represents a node in the dependency graph for path finding.
type depNode struct {
	name    string
	version string
}

func (n depNode) String() string {
	if n.version != "" {
		return n.name + "@" + n.version
	}
	return n.name
}

func runWhy(cmd *cobra.Command, args []string) error {
	// Check if command is disabled
	if err := cli.CheckCommandAllowed("why"); err != nil {
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("module name required")
	}

	targetModule := args[0]

	f, err := module.FindAndLoad()
	if err != nil {
		return err
	}

	ctx := cmdContext(cmd)

	// Create network-aware registry
	reg, err := createNetworkAwareRegistry()
	if err != nil {
		return err
	}

	// Get root module name
	rootName := f.Name()
	if rootName == "" {
		rootName = "root"
	}

	// Build dependency graph and find paths to target
	paths := findDependencyPaths(ctx, reg, f, rootName, targetModule)

	result := WhyResult{
		Module: targetModule,
		Found:  len(paths) > 0,
		Paths:  paths,
	}

	out := cmd.OutOrStdout()

	if whyJSON {
		return printWhyJSON(out, result)
	}

	return printWhyText(out, result, rootName)
}

// findDependencyPaths finds all paths from root to target module using BFS.
func findDependencyPaths(ctx context.Context, reg registry.Registry, f *module.File, rootName, targetModule string) [][]string {
	var allPaths [][]string

	// Check if target is a direct dependency
	for _, dep := range f.Deps {
		if dep.Name.String() == targetModule {
			path := []string{rootName, dep.Name.String() + "@" + dep.Version.String()}
			allPaths = append(allPaths, path)
		}
	}

	// If direct dependency found and not showing all paths, return early
	if len(allPaths) > 0 && !whyAll {
		return allPaths
	}

	// BFS to find paths through transitive dependencies
	// Each queue item is a path from root to current node
	type queueItem struct {
		path    []string
		current depNode
	}

	// Start with direct dependencies
	var queue []queueItem
	visited := make(map[string]bool)

	for _, dep := range f.Deps {
		node := depNode{name: dep.Name.String(), version: dep.Version.String()}
		path := []string{rootName, node.String()}
		queue = append(queue, queueItem{path: path, current: node})
	}

	// BFS
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		nodeKey := item.current.name + "@" + item.current.version

		// Skip if already visited (to avoid cycles)
		if visited[nodeKey] {
			continue
		}
		visited[nodeKey] = true

		// Fetch dependencies of current node
		content, err := reg.GetModuleBazel(ctx, item.current.name, item.current.version)
		if err != nil {
			continue
		}

		modFile, err := module.LoadContent(item.current.name, content)
		if err != nil {
			continue
		}

		// Check each dependency
		for _, dep := range modFile.Deps {
			depName := dep.Name.String()
			depVersion := dep.Version.String()
			depNode := depNode{name: depName, version: depVersion}

			newPath := make([]string, len(item.path)+1)
			copy(newPath, item.path)
			newPath[len(item.path)] = depNode.String()

			// Check if this is our target
			if depName == targetModule {
				allPaths = append(allPaths, newPath)

				// If not showing all paths and we found one, we can stop
				// But we should continue BFS at current level to find shortest paths
				if !whyAll && len(allPaths) > 0 {
					// For shortest paths only, find all paths of same length
					continue
				}
			}

			// Add to queue for further exploration
			queue = append(queue, queueItem{path: newPath, current: depNode})
		}
	}

	// If showing only shortest paths, filter to minimum length
	if !whyAll && len(allPaths) > 1 {
		minLen := len(allPaths[0])
		for _, p := range allPaths {
			if len(p) < minLen {
				minLen = len(p)
			}
		}

		var shortestPaths [][]string
		for _, p := range allPaths {
			if len(p) == minLen {
				shortestPaths = append(shortestPaths, p)
			}
		}
		return shortestPaths
	}

	return allPaths
}

func printWhyJSON(w io.Writer, result WhyResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func printWhyText(w io.Writer, result WhyResult, rootName string) error {
	if !result.Found {
		fmt.Fprintf(w, "%s is not a dependency of %s\n", result.Module, rootName)
		return nil
	}

	// Check if it's a direct dependency (path length of 2: root -> target)
	isDirect := false
	for _, path := range result.Paths {
		if len(path) == 2 {
			isDirect = true
			break
		}
	}

	if isDirect {
		fmt.Fprintf(w, "%s is a direct dependency\n", result.Module)
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s is required by:\n", result.Module)
	for _, path := range result.Paths {
		fmt.Fprintf(w, "  %s\n", formatPath(path))
	}

	return nil
}

// formatPath formats a path as "a -> b -> c"
func formatPath(path []string) string {
	if len(path) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString(path[0])
	for i := 1; i < len(path); i++ {
		result.WriteString(" -> " + path[i])
	}
	return result.String()
}
