package mod

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/module"
)

var (
	addDev bool
)

var addCmd = &cobra.Command{
	Use:   "add <module>@<version> [<module>@<version>...]",
	Short: "Add dependencies to MODULE.bazel",
	Long: `Add one or more Bazel module dependencies to your MODULE.bazel file.

Examples:
  bz mod add rules_go@0.50.1
  bz mod add rules_go@0.50.1 rules_python@0.35.0
  bz mod add --dev gazelle@0.38.0`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().BoolVar(&addDev, "dev", false, "Add as dev dependency")
	Cmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	modulePath, err := module.Find()
	if err != nil {
		return err
	}

	f, err := module.Load(modulePath)
	if err != nil {
		return err
	}

	// Build set of existing deps
	existingDeps := make(map[string]bool)
	for _, dep := range f.Deps {
		existingDeps[dep.Name.String()] = true
	}

	// Parse and validate modules to add
	type depToAdd struct {
		name    string
		version string
	}
	var toAdd []depToAdd

	for _, arg := range args {
		name, version := parseModuleArg(arg)

		if version == "" {
			return fmt.Errorf("version required: use %s@<version>", name)
		}

		if existingDeps[name] {
			fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s: already exists in MODULE.bazel\n", name)
			continue
		}

		toAdd = append(toAdd, depToAdd{name: name, version: version})
	}

	if len(toAdd) == 0 {
		return nil
	}

	// Read current file
	content, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	// Build new bazel_dep lines
	var newDeps strings.Builder
	for _, dep := range toAdd {
		newDeps.WriteString(module.FormatBazelDep(dep.name, dep.version, addDev))
	}

	// Append to file
	newContent := string(content)
	if len(newContent) > 0 && newContent[len(newContent)-1] != '\n' {
		newContent += "\n"
	}
	newContent += newDeps.String()

	if err := os.WriteFile(modulePath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	for _, dep := range toAdd {
		fmt.Fprintf(cmd.OutOrStdout(), "Added %s@%s\n", dep.name, dep.version)
	}

	return nil
}

// parseModuleArg splits "module@version" into name and version
func parseModuleArg(arg string) (name, version string) {
	// Handle @scope/pkg@version format
	lastAt := strings.LastIndex(arg, "@")
	if lastAt == -1 || lastAt == 0 {
		return arg, ""
	}

	// Check if the @ is part of a scoped package name
	if arg[0] == '@' && strings.Count(arg, "@") == 1 {
		return arg, ""
	}

	return arg[:lastAt], arg[lastAt+1:]
}
