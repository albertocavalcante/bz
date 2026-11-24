package mod

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

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
	modulePath, err := module.Find()
	if err != nil {
		return err
	}

	ws, err := module.OpenWorkspace(modulePath)
	if err != nil {
		return err
	}
	defer ws.Close()
	f := ws.File

	// Build set of modules to remove
	toRemove := make(map[string]bool)
	for _, arg := range args {
		toRemove[arg] = true
		if err := ws.CheckDeclaredOnce(arg); err != nil {
			return err
		}
	}

	// Find which deps exist, where they are declared and on which lines.
	var depsToRemove []depInfo
	found := make(map[string]bool)

	for _, dep := range f.UniqueDeps() {
		name := dep.Name.String()
		if toRemove[name] {
			o := f.OriginOf(dep)
			depsToRemove = append(depsToRemove, depInfo{name: name, file: ws.RelPath(o), line: o.Pos.Start.Line})
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

	rootFile := filepath.Base(modulePath)

	// Handle dry-run
	if rmDryRun {
		for _, dep := range depsToRemove {
			if dep.file == rootFile {
				fmt.Fprintf(out, "Would remove %s (line %d)\n", dep.name, dep.line)
			} else {
				fmt.Fprintf(out, "Would remove %s (%s line %d)\n", dep.name, dep.file, dep.line)
			}
		}
		return nil
	}

	edits, err := removalEdits(ws, depsToRemove)
	if err != nil {
		return err
	}
	if err := ws.WriteAll(edits); err != nil {
		return err
	}

	reported := make(map[string]bool)
	for _, dep := range depsToRemove {
		if reported[dep.name] {
			continue
		}
		reported[dep.name] = true
		if dep.file == rootFile {
			fmt.Fprintf(out, "Removed %s\n", dep.name)
		} else {
			fmt.Fprintf(out, "Removed %s (from %s)\n", dep.name, dep.file)
		}
		for _, ov := range f.OverridesFor(dep.name) {
			where := rootFile
			if ov.DeclaredIn != "" {
				where = ov.DeclaredIn
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s_override for removed module %s is still in %s\n", ov.Type, dep.name, where)
		}
	}

	return nil
}

// depInfo is a bazel_dep to remove and where it is declared.
type depInfo struct {
	name string
	file string
	line int
}

// removalEdits computes every affected file's new bytes before anything is
// written.
func removalEdits(ws *module.Workspace, deps []depInfo) ([]module.FileEdit, error) {
	var files []string
	namesByFile := make(map[string][]string)
	for _, dep := range deps {
		if !slices.Contains(namesByFile[dep.file], dep.name) {
			namesByFile[dep.file] = append(namesByFile[dep.file], dep.name)
		}
		if !slices.Contains(files, dep.file) {
			files = append(files, dep.file)
		}
	}
	edits := make([]module.FileEdit, 0, len(files))
	for _, file := range files {
		content, err := ws.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file, err)
		}
		newContent, _, err := module.RemoveDeps(content, namesByFile[file])
		if err != nil {
			return nil, fmt.Errorf("failed to remove from %s: %w", file, err)
		}
		edits = append(edits, module.FileEdit{Rel: file, Original: content, Updated: newContent})
	}
	return edits, nil
}
