// Package registry provides subcommands for registry operations.
package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/registry"
)

// cmdContext returns the command's context, defaulting to context.Background()
// if none has been set. This is needed because cobra only sets a context when
// running through Execute(); tests that call RunE directly may have a nil context.
func cmdContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// Cmd is the root command for registry operations
var Cmd = &cobra.Command{
	Use:   "registry",
	Short: "Registry operations",
	Long: `Operations for interacting with Bazel module registries.

Commands for checking registry health, connectivity, and status.`,
	// Enable typo suggestions with minimum edit distance of 2
	SuggestionsMinimumDistance: 2,
	// Require a subcommand
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// Unknown subcommand provided - show error with suggestions
		suggestions := cmd.SuggestionsFor(args[0])
		if len(suggestions) > 0 {
			return fmt.Errorf("unknown command %q for %q\n\nDid you mean this?\n\t%s",
				args[0], cmd.CommandPath(), strings.Join(suggestions, "\n\t"))
		}
		return fmt.Errorf("unknown command %q for %q, run '%s --help' for usage",
			args[0], cmd.CommandPath(), cmd.CommandPath())
	},
}

// Default registry URL for registry commands
var defaultRegistryURL = registry.DefaultBCR
