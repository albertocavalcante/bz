// Package mod provides subcommands for managing Bazel module dependencies.
package mod

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/registry"
)

// registryFlag is the registry URL (set via --registry flag)
var registryFlag string
var configureOnce sync.Once

func resolvedRegistryURL() string {
	if registryFlag != "" {
		return registryFlag
	}
	if global := cli.GetRegistry(); global != "" {
		return global
	}
	return registry.DefaultBCR
}

// createNetworkAwareRegistry creates a registry that respects offline mode settings.
// It returns the registry and any error encountered.
func createNetworkAwareRegistry() (registry.Registry, error) {
	// Create the inner registry
	inner, err := registry.New(resolvedRegistryURL())
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
	RunE: cmdutil.RequireSubcommand,
}

func configureModCmd() {
	Cmd.PersistentFlags().StringVar(&registryFlag, "registry", "", "Registry URL (https://, http://, file://, or /path)")
}

// Configure wires all `mod` subcommands once.
func Configure() {
	configureOnce.Do(func() {
		configureModCmd()
		configureAddCmd()
		configureGraphCmd()
		configureInfoCmd()
		configureLicensesCmd()
		configureListCmd()
		configureOutdatedCmd()
		configureRmCmd()
		configureSearchCmd()
		configureStatsCmd()
		configureSyncCmd()
		configureUpdateCmd()
		configureWhyCmd()

		cmdutil.ApplyErrorSilence(Cmd)
	})
}
