package cmdutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// RequireSubcommand returns help for empty args and a helpful unknown-command
// error with suggestions when an invalid subcommand is provided.
func RequireSubcommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	suggestions := cmd.SuggestionsFor(args[0])
	if len(suggestions) > 0 {
		return fmt.Errorf("unknown command %q for %q\n\nDid you mean this?\n\t%s",
			args[0], cmd.CommandPath(), strings.Join(suggestions, "\n\t"))
	}

	return fmt.Errorf("unknown command %q for %q, run '%s --help' for usage",
		args[0], cmd.CommandPath(), cmd.CommandPath())
}
