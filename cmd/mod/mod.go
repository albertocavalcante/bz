// Package mod provides subcommands for managing Bazel module dependencies.
package mod

import (
	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/registry"
)

// registryFlag is the registry URL (set via --registry flag)
var registryFlag = registry.DefaultBCR

// Cmd is the root command for module operations
var Cmd = &cobra.Command{
	Use:   "mod",
	Short: "Manage Bazel module dependencies",
	Long: `Manage Bazel module dependencies in MODULE.bazel.

Commands for adding, removing, listing, and updating bazel_dep entries.`,
}

func init() {
	Cmd.PersistentFlags().StringVar(&registryFlag, "registry", registry.DefaultBCR, "Registry URL (https://, http://, file://, or /path)")
}
