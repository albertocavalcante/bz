// Package cache provides subcommands for managing the local module cache.
package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
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

var configureOnce sync.Once

// Configure wires all `cache` subcommands once.
func Configure() {
	configureOnce.Do(func() {
		configureClearCmd()
		configureDownloadCmd()
		configureStatsCmd()
		configureVerifyCmd()
	})
}
