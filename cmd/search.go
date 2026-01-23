package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for modules in the Bazel Central Registry",
	Long: `Search the Bazel Central Registry (BCR) for available modules.

Examples:
  bz search rules_go
  bz search python`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement BCR search
		fmt.Printf("Searching BCR for: %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
