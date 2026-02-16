package mod

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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

var _ = onLoad(func() {
	rmCmd.Flags().BoolVar(&rmDryRun, "dry-run", false, "Show what would be removed without making changes")
	Cmd.AddCommand(rmCmd)
})

func runRm(cmd *cobra.Command, args []string) error {
	modulePath, err := module.Find()
	if err != nil {
		return err
	}

	f, err := module.Load(modulePath)
	if err != nil {
		return err
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
				line: dep.Pos.Line,
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

	// Build set of lines to remove
	linesToRemove := make(map[int]bool)
	for _, dep := range depsToRemove {
		linesToRemove[dep.line] = true
	}

	// Read file and filter out the lines
	file, err := os.Open(modulePath)
	if err != nil {
		return fmt.Errorf("failed to open MODULE.bazel: %w", err)
	}
	defer file.Close()

	var newLines []string
	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		if !linesToRemove[lineNum] {
			newLines = append(newLines, scanner.Text())
		}
		lineNum++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	// Handle dry-run
	if rmDryRun {
		for _, dep := range depsToRemove {
			fmt.Fprintf(out, "Would remove %s (line %d)\n", dep.name, dep.line)
		}
		return nil
	}

	// Write updated content
	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		newContent += "\n"
	}

	if err := os.WriteFile(modulePath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	for _, dep := range depsToRemove {
		fmt.Fprintf(out, "Removed %s\n", dep.name)
	}

	return nil
}
