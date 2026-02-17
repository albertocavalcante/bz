package cmdutil

import "github.com/spf13/cobra"

// ApplyErrorSilence recursively sets command error-silencing defaults.
func ApplyErrorSilence(cmd *cobra.Command) {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	for _, child := range cmd.Commands() {
		ApplyErrorSilence(child)
	}
}
