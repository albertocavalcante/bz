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

func resolvedRegistryURL() (string, error) {
	if registryFlag != "" {
		return bzconfig.NormalizeRegistry(registryFlagSource, registryFlag)
	}
	if global := cli.GetRegistry(); global != "" {
		return global, nil
	}
	return registry.DefaultBCR, nil
}

// createNetworkAwareRegistry creates a registry that respects offline mode settings.
// It returns the registry and any error encountered.
func createNetworkAwareRegistry() (registry.Registry, error) {
	registryURL, err := resolvedRegistryURL()
	if err != nil {
		return nil, err
	}

	// Create the inner registry
	inner, err := registry.New(registryURL)
	if err != nil {
		return nil, err
	}
	if inner.Type() == registry.TypeFile {
		return inner, nil
	}

	// Wrap with network-aware registry that respects CLI options
	opts := &cli.Options{
		Offline:       cli.IsEffectivelyOffline(),
		PreferOffline: cli.IsEffectivelyPreferOffline(),
	}

	// For now, we don't have a cache registry, so pass nil
	// The NetworkAwareRegistry will handle this gracefully
	return registry.NewNetworkAware(inner, nil, opts), nil
}

// registryFlagSource names the flag in registry validation errors.
const registryFlagSource = "flag --registry"

const modRootAnnotation = "bz.mod.root"

// checkRegistryFlag refuses an invalid --registry before any other work.
func checkRegistryFlag(cmd *cobra.Command) error {
	if registryFlag == "" && !cmd.Flags().Changed("registry") {
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
	Annotations: map[string]string{modRootAnnotation: "true"},
	// Enable typo suggestions with minimum edit distance of 2
	SuggestionsMinimumDistance: 2,
	// Require a subcommand
	RunE: cmdutil.RequireSubcommand,
	// Replaces the root hook for this subtree, so run it first.
	PersistentPreRunE: runModPersistentPreRun,
}

func runModPersistentPreRun(cmd *cobra.Command, args []string) error {
	if root := cmd.Root(); root.Annotations[modRootAnnotation] != "true" && root.PersistentPreRunE != nil {
		if err := root.PersistentPreRunE(cmd, args); err != nil {
			return err
		}
	}
	return checkRegistryFlag(cmd)
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
