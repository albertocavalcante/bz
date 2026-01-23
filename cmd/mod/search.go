package mod

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for modules in the registry",
	Long: `Search the Bazel Central Registry (BCR) for available modules.

Examples:
  bz mod search rules_go
  bz mod search python`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	Cmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	// TODO: implement BCR search using go-bzlmod
	query := args[0]
	fmt.Fprintf(cmd.OutOrStdout(), "Searching BCR for: %s\n", query)
	fmt.Fprintf(cmd.OutOrStdout(), "Search functionality coming soon.\n")
	return nil
}
