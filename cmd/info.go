package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <module>",
	Short: "Show information about a module",
	Long: `Show detailed information about a module from the Bazel Central Registry.

Examples:
  bz info rules_go
  bz info rules_python`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement
		fmt.Printf("Info for module: %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
