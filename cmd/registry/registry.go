// Package registry provides subcommands for registry operations.
package registry

import (
	"sync"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/registry"
)

// Cmd is the root command for registry operations
var Cmd = &cobra.Command{
	Use:   "registry",
	Short: "Registry operations",
	Long: `Operations for interacting with Bazel module registries.

Commands for checking registry health, connectivity, and status.`,
	// Enable typo suggestions with minimum edit distance of 2
	SuggestionsMinimumDistance: 2,
	// Require a subcommand
	RunE: cmdutil.RequireSubcommand,
}

// Default registry URL for registry commands
var defaultRegistryURL = registry.DefaultBCR

var configureOnce sync.Once

// Configure wires all `registry` subcommands once.
func Configure() {
	configureOnce.Do(func() {
		configurePingCmd()

		cmdutil.ApplyErrorSilence(Cmd)
	})
}
