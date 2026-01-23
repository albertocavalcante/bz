package mod

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/registry"
)

var (
	searchLimit   int
	searchVerbose bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for modules in the registry",
	Long: `Search a Bazel module registry for available modules.

The search performs fuzzy matching on module names. Results are sorted
by relevance (exact match first, then prefix matches, then alphabetical).

Supports any BCR-compatible registry:
  - HTTP/HTTPS registries (with directory listing or index.json)
  - Local filesystem registries
  - Custom registries (Artifactory, etc.)

Examples:
  bz mod search rules_go
  bz mod search python
  bz mod search protobuf -v              # Show version info
  bz mod search grpc -n 5                # Limit to 5 results
  bz mod search rules --registry=/path   # Search local registry`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 20, "Maximum number of results")
	searchCmd.Flags().BoolVarP(&searchVerbose, "verbose", "v", false, "Show version info for each result")
	Cmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Create registry from URL
	reg, err := registry.New(registryFlag)
	if err != nil {
		return fmt.Errorf("invalid registry: %w", err)
	}

	// Search
	results, err := registry.Search(ctx, reg, query)
	if err != nil {
		if errors.Is(err, registry.ErrListingNotSupported) {
			fmt.Fprintf(cmd.OutOrStdout(), "Registry %s does not support module listing.\n", reg)
			fmt.Fprintf(cmd.OutOrStdout(), "\nFor HTTP registries, ensure one of:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  - Directory listing is enabled (Apache/nginx autoindex)\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  - An index.json file exists at %s\n", registry.ModulesIndexPath())
			fmt.Fprintf(cmd.OutOrStdout(), "\nYou can still use 'bz mod info <module>' if you know the module name.\n")
			return nil
		}
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No modules found matching %q\n", query)
		return nil
	}

	// Apply limit
	total := len(results)
	if searchLimit > 0 && len(results) > searchLimit {
		results = results[:searchLimit]
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d module(s) matching %q:\n\n", len(results), query)

	for _, r := range results {
		if searchVerbose {
			// Fetch metadata for version info
			meta, err := reg.GetMetadata(ctx, r.Name)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s (error fetching metadata)\n", r.Name)
				continue
			}

			latest := meta.LatestVersion()
			info := latest
			if meta.Homepage != "" {
				homepage := meta.Homepage
				if len(homepage) > 50 {
					homepage = homepage[:47] + "..."
				}
				info = fmt.Sprintf("%s  %s", latest, homepage)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %s\n", r.Name, info)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", r.Name)
		}
	}

	if !searchVerbose && len(results) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nUse -v for version info, or 'bz mod info <module>' for details.\n")
	}

	if total > len(results) {
		fmt.Fprintf(cmd.OutOrStdout(), "\nShowing %d of %d results. Use -n to see more.\n", len(results), total)
	}

	return nil
}
