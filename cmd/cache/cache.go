// Package cache provides subcommands for managing the local module cache.
package cache

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Cmd is the root command for cache operations
var Cmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the local module cache",
	Long:  `Commands for managing the local cache for air-gapped environments.`,
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
