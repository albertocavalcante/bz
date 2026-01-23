package cmd

import (
	"fmt"
	"os"
	"strings"

	gobzlmod "github.com/albertocavalcante/go-bzlmod"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <module>[@<version>]...",
	Short: "Add a dependency to MODULE.bazel",
	Long: `Add a Bazel module dependency to your MODULE.bazel file.

Examples:
  bz add rules_go
  bz add rules_go@0.50.1
  bz add rules_python rules_rust`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAdd,
}

var (
	addDev bool
)

func init() {
	addCmd.Flags().BoolVar(&addDev, "dev", false, "Add as dev dependency")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	modulePath, err := findModuleFile()
	if err != nil {
		return err
	}

	info, err := gobzlmod.ParseModuleFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to parse MODULE.bazel: %w", err)
	}

	// Check for existing deps
	existingDeps := make(map[string]bool)
	for _, dep := range info.Dependencies {
		existingDeps[dep.Name] = true
	}

	// Parse modules to add
	var toAdd []struct{ name, version string }
	for _, arg := range args {
		name, version := parseModuleArg(arg)
		if existingDeps[name] {
			fmt.Printf("Skipping %s: already exists in MODULE.bazel\n", name)
			continue
		}
		if version == "" {
			// TODO: fetch latest version from BCR
			return fmt.Errorf("please specify version for %s: %s@<version>", name, name)
		}
		toAdd = append(toAdd, struct{ name, version string }{name, version})
	}

	if len(toAdd) == 0 {
		return nil
	}

	// Read current file
	content, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}

	// Build new bazel_dep lines
	var newDeps strings.Builder
	for _, dep := range toAdd {
		if addDev {
			fmt.Fprintf(&newDeps, "bazel_dep(name = \"%s\", version = \"%s\", dev_dependency = True)\n", dep.name, dep.version)
		} else {
			fmt.Fprintf(&newDeps, "bazel_dep(name = \"%s\", version = \"%s\")\n", dep.name, dep.version)
		}
	}

	// Append to file (simple approach - could be smarter about placement)
	newContent := string(content)
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += newDeps.String()

	if err := os.WriteFile(modulePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	for _, dep := range toAdd {
		fmt.Printf("Added %s@%s\n", dep.name, dep.version)
	}

	return nil
}
