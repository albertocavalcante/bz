package mod

import (
	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/module"
)

var (
	listJSON bool
	listAll  bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List dependencies in MODULE.bazel",
	Long: `List all bazel_dep entries in your MODULE.bazel file.

Use --all to also show extensions, overrides, and toolchains.

Examples:
  bz mod list
  bz mod list --all
  bz mod list --json`,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false, "Show all contents (extensions, overrides, etc.)")
	Cmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	f, err := module.FindAndLoad()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if listJSON {
		return f.WriteJSON(out)
	}

	if listAll {
		return f.WriteFullTable(out)
	}

	return f.WriteDepsTable(out)
}
