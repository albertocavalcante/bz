package cmd

import (
	"github.com/spf13/cobra"

	itu "github.com/albertocavalcante/bz/internal/tui"
)

var (
	tuiHeadless   bool
	runTUI        = itu.Run
	runHeadlessUI = itu.RunHeadless
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal UI",
	Long: `Launches the interactive terminal UI for browsing local dependencies
and registry search results.

If MODULE.bazel is missing, use the in-app prompt (press n) to create it.
Use --headless to run a non-interactive output mode suitable for scripts.`,
	RunE:          runTUICommand,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func runTUICommand(_ *cobra.Command, _ []string) error {
	if tuiHeadless {
		return runHeadlessUI()
	}
	return runTUI()
}

func init() {
	tuiCmd.Flags().BoolVar(&tuiHeadless, "headless", false, "Run in headless (non-interactive) mode")
	rootCmd.AddCommand(tuiCmd)
}
