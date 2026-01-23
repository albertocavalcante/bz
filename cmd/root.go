package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bz",
	Short: "CLI for Bzlmod - Bazel's module system",
	Long: `bz is a CLI tool for managing Bazel modules (Bzlmod).

It helps you manage MODULE.bazel dependencies, query the Bazel Central Registry,
and streamline your Bazel module workflow.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}
