package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <module>[@<version>]",
	Short: "Add a dependency to MODULE.bazel",
	Long: `Add a Bazel module dependency to your MODULE.bazel file.

Examples:
  bz add rules_go
  bz add rules_go@0.50.1
  bz add rules_python rules_rust`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: implement
		fmt.Printf("Adding dependencies: %v\n", args)
		return nil
	},
}

var (
	addDev bool
)

func init() {
	addCmd.Flags().BoolVar(&addDev, "dev", false, "Add as dev dependency")
	rootCmd.AddCommand(addCmd)
}
