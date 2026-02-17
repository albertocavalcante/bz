package mod

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
)

var (
	infoJSON bool
)

var infoCmd = &cobra.Command{
	Use:   "info <module>[@<version>]",
	Short: "Show information about a module from the registry",
	Long: `Display detailed information about a Bazel module from the registry.

If no version is specified, shows metadata including all available versions.
If a version is specified, shows the MODULE.bazel content for that version.

Examples:
  bz mod info rules_go                   # Show metadata and versions
  bz mod info rules_go@0.50.1            # Show specific version details
  bz mod info rules_go --json            # Output as JSON
  bz mod info rules_go --registry=/path  # Use local registry`,
	Args:              cobra.ExactArgs(1),
	RunE:              runInfo,
	ValidArgsFunction: completeModuleNamesForSingleArg,
}

func configureInfoCmd() {
	infoCmd.Flags().BoolVar(&infoJSON, "json", false, "Output as JSON")
	Cmd.AddCommand(infoCmd)
}

func runInfo(cmd *cobra.Command, args []string) error {
	// Check if command is disabled
	if err := cli.CheckCommandAllowed("info"); err != nil {
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("module argument required")
	}
	name, version := parseModuleArg(args[0])
	ctx := cmdutil.CommandContext(cmd)

	// Create network-aware registry
	reg, err := createNetworkAwareRegistry()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	// If no version specified, show metadata
	if version == "" {
		meta, err := reg.GetMetadata(ctx, name)
		if err != nil {
			if errors.Is(err, registry.ErrModuleNotFound) {
				return registry.WrapModuleNotFound(ctx, reg, name, err)
			}
			return fmt.Errorf("failed to fetch module metadata: %w", err)
		}

		if infoJSON {
			return printMetadataJSON(out, name, meta)
		}
		return printMetadataTable(out, name, meta)
	}

	// Version specified - fetch MODULE.bazel and parse it
	content, err := reg.GetModuleBazel(ctx, name, version)
	if err != nil {
		if errors.Is(err, registry.ErrModuleNotFound) {
			return registry.WrapModuleNotFound(ctx, reg, name, err)
		}
		return fmt.Errorf("failed to fetch module: %w", err)
	}

	// Parse the MODULE.bazel content
	modFile, err := module.LoadContent(name, content)
	if err != nil {
		return fmt.Errorf("failed to parse MODULE.bazel: %w", err)
	}

	if infoJSON {
		return printModuleJSON(out, name, version, modFile)
	}
	return printModuleTable(out, name, version, modFile)
}

func printMetadataTable(w io.Writer, name string, meta *registry.Metadata) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintf(tw, "Name:\t%s\n", name)
	if meta.Homepage != "" {
		fmt.Fprintf(tw, "Homepage:\t%s\n", meta.Homepage)
	}

	if len(meta.Repository) > 0 {
		fmt.Fprintf(tw, "Repository:\t%s\n", meta.Repository[0])
	}

	latest := meta.LatestVersion()
	if latest != "" {
		fmt.Fprintf(tw, "Latest:\t%s\n", latest)
	}

	fmt.Fprintf(tw, "Versions:\t%d available\n", len(meta.Versions))

	// Show recent versions
	if len(meta.Versions) > 0 {
		fmt.Fprintln(tw)
		fmt.Fprintln(tw, "Recent versions:")
		start := max(len(meta.Versions)-5, 0)
		for i := len(meta.Versions) - 1; i >= start; i-- {
			v := meta.Versions[i]
			yanked := ""
			if reason, ok := meta.YankedVersions[v]; ok {
				yanked = fmt.Sprintf(" (yanked: %s)", reason)
			}
			fmt.Fprintf(tw, "  %s%s\n", v, yanked)
		}
	}

	if len(meta.Maintainers) > 0 {
		fmt.Fprintln(tw)
		fmt.Fprintln(tw, "Maintainers:")
		for _, m := range meta.Maintainers {
			if m.GitHub != "" {
				fmt.Fprintf(tw, "  @%s", m.GitHub)
			} else if m.Name != "" {
				fmt.Fprintf(tw, "  %s", m.Name)
			}
			if m.Email != "" {
				fmt.Fprintf(tw, " <%s>", m.Email)
			}
			fmt.Fprintln(tw)
		}
	}

	return tw.Flush()
}

func printMetadataJSON(w io.Writer, name string, meta *registry.Metadata) error {
	output := struct {
		Name     string             `json:"name"`
		Metadata *registry.Metadata `json:"metadata"`
	}{
		Name:     name,
		Metadata: meta,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func printModuleTable(w io.Writer, name, version string, mod *module.File) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintf(tw, "Name:\t%s\n", name)
	fmt.Fprintf(tw, "Version:\t%s\n", version)

	if mod.Name() != "" && mod.Name() != name {
		fmt.Fprintf(tw, "Module Name:\t%s\n", mod.Name())
	}

	if len(mod.Deps) > 0 {
		fmt.Fprintln(tw)
		fmt.Fprintln(tw, "Dependencies:")
		for _, dep := range mod.Deps {
			devStr := ""
			if dep.DevDependency {
				devStr = " (dev)"
			}
			fmt.Fprintf(tw, "  %s@%s%s\n", dep.Name, dep.Version, devStr)
		}
	}

	return tw.Flush()
}

func printModuleJSON(w io.Writer, name, version string, mod *module.File) error {
	// Build a simplified output structure
	deps := make([]map[string]any, 0, len(mod.Deps))
	for _, dep := range mod.Deps {
		d := map[string]any{
			"name":    dep.Name.String(),
			"version": dep.Version.String(),
		}
		if dep.DevDependency {
			d["dev_dependency"] = true
		}
		deps = append(deps, d)
	}

	output := map[string]any{
		"name":         name,
		"version":      version,
		"dependencies": deps,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
