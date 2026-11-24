package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cmdutil"
	bzversion "github.com/albertocavalcante/bz/internal/version"
)

// Set at build time via -ldflags "-X github.com/albertocavalcante/bz/cmd.version=..."
// (and likewise commit, date). When unset, buildInfo falls back to the metadata
// embedded by the Go toolchain; see internal/version.Resolve.
var (
	version = bzversion.UnsetVersion
	commit  = bzversion.UnsetCommit
	date    = bzversion.UnsetDate
)

func buildInfo() bzversion.Info {
	return bzversion.Current(version, commit, date)
}

var versionJSON bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		info := buildInfo()
		if versionJSON {
			payload := struct {
				Version   string `json:"version"`
				Commit    string `json:"commit"`
				BuildDate string `json:"build_date"`
			}{
				Version:   info.Version,
				Commit:    info.Commit,
				BuildDate: info.Date,
			}
			_ = cmdutil.WriteJSON(out, payload)
			return
		}
		fmt.Fprintf(out, "bz %s (%s) built on %s\n", info.Version, info.Commit, info.Date)
	},
}

func configureVersionCmd() {
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(versionCmd)
}
