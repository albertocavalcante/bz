// Package cache provides subcommands for managing the local module cache.
package cache

import (
	"sync"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cmdutil"
)

// Cmd is the root command for cache operations
var Cmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the local module cache",
	Long:  `Commands for managing the local cache for air-gapped environments.`,
	// Enable typo suggestions with minimum edit distance of 2
	SuggestionsMinimumDistance: 2,
	// Require a subcommand
	RunE: cmdutil.RequireSubcommand,
}

var configureOnce sync.Once

// Configure wires all `cache` subcommands once.
func Configure() {
	configureOnce.Do(func() {
		configureClearCmd()
		configureDownloadCmd()
		configureStatsCmd()
		configureVerifyCmd()

		cmdutil.ApplyErrorSilence(Cmd)
	})
}
