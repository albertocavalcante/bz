package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
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
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			_ = enc.Encode(info)
			return
		}
		fmt.Fprintf(out, "bz %s (%s) built on %s\n", version, commit, date)
	},
}

func init() {
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(versionCmd)
}
