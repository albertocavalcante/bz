// Package registry provides subcommands for registry operations.
package registry

import (
	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/registry"
)

// Cmd is the root command for registry operations
var Cmd = &cobra.Command{
	Use:   "registry",
	Short: "Registry operations",
	Long: `Operations for interacting with Bazel module registries.

Commands for checking registry health, connectivity, and status.`,
}

// Default registry URL for registry commands
var defaultRegistryURL = registry.DefaultBCR
