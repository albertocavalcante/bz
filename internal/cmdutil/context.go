// Package cmdutil provides shared helpers for Cobra command handlers.
package cmdutil

import (
	"context"

	"github.com/spf13/cobra"
)

// CommandContext returns the command context or context.Background when unset.
// Tests that call RunE directly may not have a context set by Cobra.
func CommandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
