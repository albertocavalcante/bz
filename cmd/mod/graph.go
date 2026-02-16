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
	graphFormat string
	graphJSON   bool
	graphDepth  int
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Display module dependency graph",
	Long: `Display the dependency graph of your Bazel module.

This command parses your MODULE.bazel and recursively fetches dependencies
from the registry to build a complete dependency tree.

Output formats:
  - ascii (default): ASCII art tree
  - dot: Graphviz DOT format
  - json: JSON structure
  - mermaid: Mermaid diagram format

Examples:
  bz mod graph
  bz mod graph --format=dot | dot -Tpng -o deps.png
  bz mod graph --format=json
  bz mod graph --json
  bz mod graph --depth=2
  bz mod graph --format=mermaid`,
	RunE: runGraph,
}

var _ = onLoad(func() {
	graphCmd.Flags().StringVar(&graphFormat, "format", "ascii", "Output format (ascii, dot, json, mermaid)")
	graphCmd.Flags().BoolVar(&graphJSON, "json", false, "Output as JSON (shortcut for --format=json)")
	graphCmd.Flags().IntVar(&graphDepth, "depth", 0, "Maximum depth to traverse (0 = unlimited)")
	Cmd.AddCommand(graphCmd)
})

// GraphNode represents a node in the dependency graph.
type GraphNode struct {
	Name         string       `json:"name"`
	Version      string       `json:"version,omitempty"`
	Dependencies []*GraphNode `json:"dependencies,omitempty"`
	IsCycle      bool         `json:"is_cycle,omitempty"`
}

// GraphOutput is the JSON output structure.
type GraphOutput struct {
	Root *GraphNode `json:"root"`
}

func runGraph(cmd *cobra.Command, args []string) error {
	// Check if command is disabled
	if err := cli.CheckCommandAllowed("graph"); err != nil {
		return err
	}

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

	// Build the dependency graph
	rootName := f.Name()
	if rootName == "" {
		rootName = "root"
	}

	root := &GraphNode{
		Name:    rootName,
		Version: f.Version(),
	}

	// Track visited modules to detect cycles
	visited := make(map[string]bool)
	visiting := make(map[string]bool) // Currently in recursion stack

	// Mark root as visiting to detect cycles back to root
	rootKey := rootName
	if f.Version() != "" {
		rootKey = rootName + "@" + f.Version()
	}
	visiting[rootKey] = true
	// Also mark just the name (without version) to detect cycles to root by name
	visiting[rootName] = true

	// Build graph recursively with spinner
	err = cli.WithSpinner("Building dependency graph...", func() error {
		for _, dep := range f.Deps {
			name := dep.Name.String()
			version := dep.Version.String()
			child := buildGraph(ctx, reg, name, version, visited, visiting, 1)
			root.Dependencies = append(root.Dependencies, child)
		}
		return nil
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	// Determine output format
	format := graphFormat
	if graphJSON {
		format = "json"
	}

	switch format {
	case "ascii":
		return printASCIITree(out, root)
	case "dot":
		return printDOTGraph(out, root)
	case "json":
		return printJSONGraph(out, root)
	case "mermaid":
		return printMermaidGraph(out, root)
	default:
		return fmt.Errorf("unknown format: %s (valid formats: ascii, dot, json, mermaid)", format)
	}
}

// buildGraph recursively builds the dependency graph.
func buildGraph(ctx context.Context, reg registry.Registry, name, version string, visited, visiting map[string]bool, depth int) *GraphNode {
	key := name + "@" + version
	node := &GraphNode{
		Name:    name,
		Version: version,
	}

	// Check for cycle (in current recursion stack)
	// Check both the exact key and just the module name (for root module cycles)
	if visiting[key] || visiting[name] {
		node.IsCycle = true
		return node
	}

	// Check depth limit - don't recurse beyond this depth
	if graphDepth > 0 && depth > graphDepth {
		return node
	}

	// Mark as being visited (in current recursion)
	visiting[key] = true
	defer func() { visiting[key] = false }()

	// If already fully visited, we still need to show it but don't recurse
	if visited[key] {
		return node
	}
	visited[key] = true

	// Fetch module dependencies from registry
	content, err := reg.GetModuleBazel(ctx, name, version)
	if err != nil {
		// Module not found in registry - just return the node without children
		return node
	}

	modFile, err := module.LoadContent(name, content)
	if err != nil {
		return node
	}

	// Add dependencies (only if we haven't reached the depth limit)
	// Depth limit check: if graphDepth > 0 and we're at max depth, don't add children
	if graphDepth == 0 || depth < graphDepth {
		for _, dep := range modFile.Deps {
			depName := dep.Name.String()
			depVersion := dep.Version.String()
			child := buildGraph(ctx, reg, depName, depVersion, visited, visiting, depth+1)
			node.Dependencies = append(node.Dependencies, child)
		}
	}

	return node
}

// printASCIITree prints the graph as an ASCII tree.
func printASCIITree(w io.Writer, root *GraphNode) error {
	fmt.Fprintln(w, root.Name)
	printASCIIChildren(w, root.Dependencies, "")
	return nil
}

func printASCIIChildren(w io.Writer, children []*GraphNode, prefix string) {
	for i, child := range children {
		isLast := i == len(children)-1

		// Print connector
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		// Format node name
		nodeName := child.Name
		if child.Version != "" {
			nodeName += "@" + child.Version
		}
		if child.IsCycle {
			nodeName += " (cycle)"
		}

		fmt.Fprintf(w, "%s%s%s\n", prefix, connector, nodeName)

		// Print children with updated prefix
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
		printASCIIChildren(w, child.Dependencies, childPrefix)
	}
}

// printDOTGraph prints the graph in Graphviz DOT format.
func printDOTGraph(w io.Writer, root *GraphNode) error {
	fmt.Fprintln(w, "digraph dependencies {")
	fmt.Fprintln(w, "    rankdir=TB;")
	fmt.Fprintln(w, "    node [shape=box];")
	fmt.Fprintln(w)

	// Track edges to avoid duplicates
	edges := make(map[string]bool)
	printDOTNode(w, root, edges)

	fmt.Fprintln(w, "}")
	return nil
}

func printDOTNode(w io.Writer, node *GraphNode, edges map[string]bool) {
	nodeName := formatNodeName(node)
	nodeID := sanitizeDOTID(nodeName)

	for _, child := range node.Dependencies {
		childName := formatNodeName(child)
		childID := sanitizeDOTID(childName)

		edgeKey := nodeID + "->" + childID
		if !edges[edgeKey] {
			edges[edgeKey] = true
			style := ""
			if child.IsCycle {
				style = " [style=dashed, color=red]"
			}
			fmt.Fprintf(w, "    \"%s\" -> \"%s\"%s;\n", nodeName, childName, style)
		}

		if !child.IsCycle {
			printDOTNode(w, child, edges)
		}
	}
}

func sanitizeDOTID(s string) string {
	// Replace special characters for DOT node IDs
	s = strings.ReplaceAll(s, "@", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func formatNodeName(node *GraphNode) string {
	if node.Version != "" {
		return node.Name + "@" + node.Version
	}
	return node.Name
}

// printJSONGraph prints the graph as JSON.
func printJSONGraph(w io.Writer, root *GraphNode) error {
	output := GraphOutput{Root: root}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// printMermaidGraph prints the graph in Mermaid format.
func printMermaidGraph(w io.Writer, root *GraphNode) error {
	fmt.Fprintln(w, "graph TD")

	// Track edges to avoid duplicates
	edges := make(map[string]bool)
	printMermaidNode(w, root, edges)

	return nil
}

func printMermaidNode(w io.Writer, node *GraphNode, edges map[string]bool) {
	nodeName := formatNodeName(node)
	nodeID := sanitizeMermaidID(nodeName)

	for _, child := range node.Dependencies {
		childName := formatNodeName(child)
		childID := sanitizeMermaidID(childName)

		edgeKey := nodeID + "-->" + childID
		if !edges[edgeKey] {
			edges[edgeKey] = true
			fmt.Fprintf(w, "    %s[\"%s\"] --> %s[\"%s\"]\n", nodeID, nodeName, childID, childName)
		}

		if !child.IsCycle {
			printMermaidNode(w, child, edges)
		}
	}
}

func sanitizeMermaidID(s string) string {
	// Replace special characters for Mermaid node IDs
	s = strings.ReplaceAll(s, "@", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}
