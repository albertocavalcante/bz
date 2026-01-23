package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List dependencies in MODULE.bazel",
	Long:  `List all bazel_dep entries in your MODULE.bazel file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement
		fmt.Println("Listing dependencies...")
		return nil
	},
}

var (
	listOutdated bool
)

func init() {
	listCmd.Flags().BoolVar(&listOutdated, "outdated", false, "Show only outdated dependencies")
	rootCmd.AddCommand(listCmd)
}
