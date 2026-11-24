// Package mod provides subcommands for managing Bazel module dependencies.
package mod

import (
	"sync"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/bzconfig"
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
	if err := checkRegistryFlag(); err != nil {
		return nil, err
	}

	// Create the inner registry
	inner, err := registry.New(resolvedRegistryURL())
	if err != nil {
		return nil, err
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

// registryFlagSource names the flag in registry validation errors.
const registryFlagSource = "flag --registry"

// checkRegistryFlag refuses an invalid --registry before any other work.
func checkRegistryFlag() error {
	if registryFlag == "" {
		return nil
	}
	return bzconfig.ValidateRegistry(registryFlagSource, registryFlag)
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
	// Replaces the root hook for this subtree, so run it first.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if root := cmd.Root(); root.PersistentPreRunE != nil {
			if err := root.PersistentPreRunE(cmd, args); err != nil {
				return err
			}
		}
		return checkRegistryFlag()
	},
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
