package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cmdutil"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionJSON bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		if versionJSON {
			info := struct {
				Version   string `json:"version"`
				Commit    string `json:"commit"`
				BuildDate string `json:"build_date"`
			}{
				Version:   version,
				Commit:    commit,
				BuildDate: date,
			}
			_ = cmdutil.WriteJSON(out, info)
			return
		}
		fmt.Fprintf(out, "bz %s (%s) built on %s\n", version, commit, date)
	},
}

func configureVersionCmd() {
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(versionCmd)
}
