// Package mod provides subcommands for managing Bazel module dependencies.
package mod

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/registry"
)

// registryFlag is the registry URL (set via --registry flag)
var registryFlag = registry.DefaultBCR

// createNetworkAwareRegistry creates a registry that respects offline mode settings.
// It returns the registry and any error encountered.
func createNetworkAwareRegistry() (registry.Registry, error) {
	// Create the inner registry
	inner, err := registry.New(registryFlag)
	if err != nil {
		return nil, fmt.Errorf("invalid registry: %w", err)
	}

	// Wrap with network-aware registry that respects CLI options
	opts := &cli.Options{
		Offline:       cli.IsOffline(),
		PreferOffline: cli.IsPreferOffline(),
	}

	// For now, we don't have a cache registry, so pass nil
	// The NetworkAwareRegistry will handle this gracefully
	return registry.NewNetworkAware(inner, nil, opts), nil
}

// Cmd is the root command for module operations
var Cmd = &cobra.Command{
	Use:   "mod",
	Short: "Manage Bazel module dependencies",
	Long: `Manage Bazel module dependencies in MODULE.bazel.

Commands for adding, removing, listing, and updating bazel_dep entries.`,
	// Enable typo suggestions with minimum edit distance of 2
	SuggestionsMinimumDistance: 2,
	// Require a subcommand
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// Unknown subcommand provided - show error with suggestions
		suggestions := cmd.SuggestionsFor(args[0])
		if len(suggestions) > 0 {
			return fmt.Errorf("unknown command %q for %q\n\nDid you mean this?\n\t%s",
				args[0], cmd.CommandPath(), strings.Join(suggestions, "\n\t"))
		}
		return fmt.Errorf("unknown command %q for %q, run '%s --help' for usage",
			args[0], cmd.CommandPath(), cmd.CommandPath())
	},
}

func init() {
	Cmd.PersistentFlags().StringVar(&registryFlag, "registry", registry.DefaultBCR, "Registry URL (https://, http://, file://, or /path)")
}
