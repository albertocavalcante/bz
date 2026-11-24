package mod

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/module"
)

var (
	rmDryRun bool
)

var rmCmd = &cobra.Command{
	Use:   "rm <module> [<module>...]",
	Short: "Remove dependencies from MODULE.bazel",
	Long: `Remove one or more Bazel module dependencies from your MODULE.bazel file.

Examples:
  bz mod rm rules_go
  bz mod rm rules_go rules_python
  bz mod rm --dry-run rules_go`,
	Args:              cobra.MinimumNArgs(1),
	RunE:              runRm,
	ValidArgsFunction: completeInstalledModules,
}

func configureRmCmd() {
	rmCmd.Flags().BoolVar(&rmDryRun, "dry-run", false, "Show what would be removed without making changes")
	Cmd.AddCommand(rmCmd)
}

func runRm(cmd *cobra.Command, args []string) error {
	if err := cli.CheckCommandAllowed("rm"); err != nil {
		return err
	}

	modulePath, err := module.Find()
	if err != nil {
		return err
	}

	content, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	f, err := module.LoadContent(modulePath, content)
	if err != nil {
		return err
	}

	included, err := includedDeps(f, modulePath)
	if err != nil {
		return err
	}
	for _, arg := range args {
		if o, ok := included[arg]; ok {
			return includedEditError(arg, o)
		}
	}

	// Build set of modules to remove
	toRemove := make(map[string]bool)
	for _, arg := range args {
		toRemove[arg] = true
	}

	// Find which deps exist and which lines they're on
	type depInfo struct {
		name string
		line int
	}
	var depsToRemove []depInfo
	found := make(map[string]bool)

	for _, dep := range f.Deps {
		name := dep.Name.String()
		if toRemove[name] {
			depsToRemove = append(depsToRemove, depInfo{
				name: name,
				line: dep.Pos.Start.Line,
			})
			found[name] = true
		}
	}

	// Warn about modules not found
	out := cmd.OutOrStdout()
	for _, arg := range args {
		if !found[arg] {
			fmt.Fprintf(out, "Warning: %s not found in MODULE.bazel\n", arg)
		}
	}

	if len(depsToRemove) == 0 {
		return nil
	}

	// Handle dry-run
	if rmDryRun {
		for _, dep := range depsToRemove {
			fmt.Fprintf(out, "Would remove %s (line %d)\n", dep.name, dep.line)
		}
		return nil
	}

	names := make([]string, 0, len(depsToRemove))
	for _, dep := range depsToRemove {
		names = append(names, dep.name)
	}
	newContent, removed, err := module.RemoveDeps(content, names)
	if err != nil {
		return fmt.Errorf("failed to remove from MODULE.bazel: %w", err)
	}

	if err := module.WriteFileIfUnchanged(modulePath, content, newContent); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	for _, name := range removed {
		fmt.Fprintf(out, "Removed %s\n", name)
		for _, ov := range f.OverridesFor(name) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s_override for removed module %s is still in MODULE.bazel\n", ov.Type, name)
		}
	}

	return nil
}
