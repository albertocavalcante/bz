package mod

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/version"
)

var (
	outdatedJSON bool
)

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Show dependencies with newer versions available",
	Long: `Check all dependencies in MODULE.bazel for newer versions.

Compares each dependency's current version against the latest available
version in the registry and shows which ones can be updated.

Examples:
  bz mod outdated
  bz mod outdated --json
  bz mod outdated --registry=https://my-registry.example.com`,
	RunE: runOutdated,
}

func configureOutdatedCmd() {
	outdatedCmd.Flags().BoolVar(&outdatedJSON, "json", false, "Output as JSON")
	Cmd.AddCommand(outdatedCmd)
}

// OutdatedDep represents a dependency with version comparison info.
type OutdatedDep struct {
	Name       string             `json:"name"`
	Current    string             `json:"current"`
	Latest     string             `json:"latest"`
	UpdateType version.UpdateType `json:"update_type"`
	Error      string             `json:"error,omitempty"`
}

func runOutdated(cmd *cobra.Command, args []string) error {
	// Check if command is disabled
	if err := cli.CheckCommandAllowed("outdated"); err != nil {
		return err
	}

	f, err := module.FindAndLoad()
	if err != nil {
		return err
	}

	if len(f.Deps) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No dependencies found in MODULE.bazel")
		return nil
	}

	ctx := cmdutil.CommandContext(cmd)

	// Create network-aware registry
	reg, err := createNetworkAwareRegistry()
	if err != nil {
		return err
	}

	// Check each dependency with spinner
	var deps []OutdatedDep
	checkErr := cli.WithSpinner("Checking for updates...", func() error {
		for _, dep := range f.Deps {
			name := dep.Name.String()
			current := dep.Version.String()

			od := OutdatedDep{
				Name:    name,
				Current: current,
			}

			// Fetch metadata from registry
			meta, err := reg.GetMetadata(ctx, name)
			if err != nil {
				od.Error = err.Error()
				od.Latest = current
				od.UpdateType = version.None
				deps = append(deps, od)
				continue
			}

			latest := meta.LatestVersion()
			if latest == "" {
				latest = current
			}

			od.Latest = latest
			od.UpdateType = version.ClassifyUpdate(current, latest)
			deps = append(deps, od)
		}
		return nil
	})
	if checkErr != nil {
		return checkErr
	}

	out := cmd.OutOrStdout()

	if outdatedJSON {
		return printOutdatedJSON(out, deps)
	}

	return printOutdatedTable(out, deps)
}

func printOutdatedTable(w io.Writer, deps []OutdatedDep) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintln(tw, "Module\tCurrent\tLatest\tUpdate")
	fmt.Fprintln(tw, "------\t-------\t------\t------")

	hasUpdates := false
	for _, dep := range deps {
		if dep.Error != "" {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", dep.Name, dep.Current, cli.Error("error"), dep.Error)
			continue
		}

		updateStr := colorizeUpdateType(dep.UpdateType)
		if dep.UpdateType != version.None {
			hasUpdates = true
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", dep.Name, dep.Current, dep.Latest, updateStr)
	}

	if err := tw.Flush(); err != nil {
		return err
	}

	if !hasUpdates {
		fmt.Fprintln(w, "\n"+cli.Success("All dependencies are up to date!"))
	}

	return nil
}

// colorizeUpdateType returns the update type string with appropriate color.
func colorizeUpdateType(ut version.UpdateType) string {
	switch ut {
	case version.Major:
		return cli.Error(ut.String()) // Red for breaking changes
	case version.Minor:
		return cli.Warning(ut.String()) // Yellow for new features
	case version.Patch:
		return cli.Success(ut.String()) // Green for bug fixes
	default:
		return ut.String()
	}
}

func printOutdatedJSON(w io.Writer, deps []OutdatedDep) error {
	// Filter to only include dependencies with updates available
	var outdated []OutdatedDep
	for _, dep := range deps {
		if dep.UpdateType != version.None || dep.Error != "" {
			outdated = append(outdated, dep)
		}
	}

	output := struct {
		Total    int           `json:"total"`
		Outdated int           `json:"outdated"`
		Deps     []OutdatedDep `json:"dependencies"`
	}{
		Total:    len(deps),
		Outdated: len(outdated),
		Deps:     deps,
	}

	return cmdutil.WriteJSON(w, output)
}
