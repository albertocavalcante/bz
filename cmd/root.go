package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/cmd/mod"
	"github.com/albertocavalcante/bz/cmd/registry"
	"github.com/albertocavalcante/bz/internal/cli"
)

var rootCmd = &cobra.Command{
	Use:   "bz",
	Short: "CLI for Bzlmod - Bazel's module system",
	Long: `bz is a CLI tool for managing Bazel modules (Bzlmod).

It helps you manage MODULE.bazel dependencies, query the Bazel Central Registry,
and streamline your Bazel module workflow.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Configure lipgloss color profile based on --no-color flag
		cli.ConfigureLipgloss()
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

	rootCmd.AddCommand(mod.Cmd)
	rootCmd.AddCommand(registry.Cmd)
}
