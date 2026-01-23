// Package mod provides subcommands for managing Bazel module dependencies.
package mod

import (
	"github.com/spf13/cobra"
)

const defaultRegistry = "https://bcr.bazel.build"

// registry is the BCR URL to use (set via --registry flag)
var registry string

// Cmd is the root command for module operations
var Cmd = &cobra.Command{
	Use:   "mod",
	Short: "Manage Bazel module dependencies",
	Long: `Manage Bazel module dependencies in MODULE.bazel.

Commands for adding, removing, listing, and updating bazel_dep entries.`,
}

func init() {
	Cmd.PersistentFlags().StringVar(&registry, "registry", defaultRegistry, "Bazel Central Registry URL")
}
