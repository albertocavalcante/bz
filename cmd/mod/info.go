package mod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	gobzlmod "github.com/albertocavalcante/go-bzlmod"
	"github.com/spf13/cobra"
)

var (
	infoJSON bool
)

var infoCmd = &cobra.Command{
	Use:   "info <module>@<version>",
	Short: "Show information about a module from the registry",
	Long: `Display detailed information about a Bazel module from the registry.

Examples:
  bz mod info rules_go@0.50.1
  bz mod info rules_go@0.50.1 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func init() {
	infoCmd.Flags().BoolVar(&infoJSON, "json", false, "Output as JSON")
	Cmd.AddCommand(infoCmd)
}

func runInfo(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("module argument required: use <module>@<version>")
	}

	name, version := parseModuleArg(args[0])

	if version == "" {
		return fmt.Errorf("version required: use %s@<version>", name)
	}

	client := gobzlmod.NewRegistryClient(registry)
	moduleInfo, err := client.GetModuleFile(context.Background(), name, version)
	if err != nil {
		return fmt.Errorf("failed to fetch module info: %w", err)
	}

	out := cmd.OutOrStdout()

	if infoJSON {
		return printInfoJSON(out, moduleInfo)
	}

	return printInfoTable(out, name, version, moduleInfo)
}

func printInfoTable(w io.Writer, name, version string, info *gobzlmod.ModuleInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintf(tw, "Name:\t%s\n", name)
	fmt.Fprintf(tw, "Version:\t%s\n", version)

	if info.Name != "" && info.Name != name {
		fmt.Fprintf(tw, "Module Name:\t%s\n", info.Name)
	}

	if len(info.Dependencies) > 0 {
		fmt.Fprintln(tw)
		fmt.Fprintln(tw, "Dependencies:")
		for _, dep := range info.Dependencies {
			dev := ""
			if dep.DevDependency {
				dev = " (dev)"
			}
			fmt.Fprintf(tw, "  %s@%s%s\n", dep.Name, dep.Version, dev)
		}
	}

	return tw.Flush()
}

func printInfoJSON(w io.Writer, info *gobzlmod.ModuleInfo) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(info)
}
