package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/cmd/cache"
	"github.com/albertocavalcante/bz/cmd/mod"
	"github.com/albertocavalcante/bz/cmd/registry"
	"github.com/albertocavalcante/bz/internal/cli"
)

var showVersion bool

var rootCmd = &cobra.Command{
	Use:   "bz",
	Short: "CLI for Bzlmod - Bazel's module system",
	Long: `bz is a CLI tool for managing Bazel modules (Bzlmod).

It helps you manage MODULE.bazel dependencies, query the Bazel Central Registry,
and streamline your Bazel module workflow.

Air-gapped/Offline Usage:
  Use --offline to disable all network access and rely solely on cached data.
  Use --prefer-offline to prefer cached data but fall back to network if needed.
  Use --registry to override the default registry URL for custom registries.

Environment Variables:
  BZ_OFFLINE=1         Same as --offline flag
  BZ_PREFER_OFFLINE=1  Same as --prefer-offline flag
  BZ_REGISTRY=<url>    Same as --registry flag`,
	// Enable typo suggestions with minimum edit distance of 2
	SuggestionsMinimumDistance: 2,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate mutually exclusive flags
		if err := cli.ValidateOfflineFlags(); err != nil {
			return err
		}
		// Configure lipgloss color profile based on --no-color flag
		cli.ConfigureLipgloss()
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			versionCmd.Run(cmd, args)
			return
		}
		cmd.Help()
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&cli.Global.Quiet, "quiet", "q", false, "Reduce output, only show errors and essential info")
	rootCmd.PersistentFlags().BoolVar(&cli.Global.NoColor, "no-color", false, "Disable colored output")

	// Network/offline flags for air-gapped environments
	rootCmd.PersistentFlags().BoolVar(&cli.Global.Offline, "offline", false, "Disable all network access, use cache only (for air-gapped environments)")
	rootCmd.PersistentFlags().BoolVar(&cli.Global.PreferOffline, "prefer-offline", false, "Prefer cached data, fallback to network if needed")
	rootCmd.PersistentFlags().StringVar(&cli.Global.Registry, "registry", "", "Override registry URL")

	// Version flag as shorthand alias for 'bz version'
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print version information")

	rootCmd.AddCommand(cache.Cmd)
	rootCmd.AddCommand(mod.Cmd)
	rootCmd.AddCommand(registry.Cmd)
}
