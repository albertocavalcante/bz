package mod

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/modulearg"
	"github.com/albertocavalcante/bz/internal/registry"
)

var (
	addDev      bool
	addNoVerify bool
	addDryRun   bool
)

var addCmd = &cobra.Command{
	Use:   "add <module>@<version> [<module>@<version>...]",
	Short: "Add dependencies to MODULE.bazel",
	Long: `Add one or more Bazel module dependencies to your MODULE.bazel file.

By default, the command validates that the module and version exist in the
registry before adding them. If the module is not found, it will suggest
similar module names. Use --no-verify to skip validation (for offline use).

Examples:
  bz mod add rules_go@0.50.1
  bz mod add rules_go@0.50.1 rules_python@0.35.0
  bz mod add --dev gazelle@0.38.0
  bz mod add --dry-run rules_go@0.50.1
  bz mod add --no-verify my_module@1.0.0   # Skip validation`,
	Args:              cobra.MinimumNArgs(1),
	RunE:              runAdd,
	ValidArgsFunction: completeModuleNames,
}

func configureAddCmd() {
	addCmd.Flags().BoolVar(&addDev, "dev", false, "Add as dev dependency")
	addCmd.Flags().BoolVar(&addNoVerify, "no-verify", false, "Skip registry validation (for offline use)")
	addCmd.Flags().BoolVar(&addDryRun, "dry-run", false, "Show what would be added without modifying MODULE.bazel")
	Cmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Check if command is disabled
	if err := cli.CheckCommandAllowed("add"); err != nil {
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
		name, _ := parseModuleArg(arg)
		if o, ok := included[name]; ok {
			return includedEditError(name, o)
		}
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

	requested := make(map[string]bool)
	for _, arg := range args {
		name, version := parseModuleArg(arg)

		if requested[name] {
			return fmt.Errorf("module %q specified more than once", name)
		}
		requested[name] = true

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

	// Validate modules exist in registry (unless --no-verify is set)
	if !addNoVerify {
		ctx := cmdutil.CommandContext(cmd)

		// Create network-aware registry
		reg, err := createNetworkAwareRegistry()
		if err != nil {
			return err
		}

		for _, dep := range toAdd {
			if err := validateModule(ctx, cmd.ErrOrStderr(), reg, dep.name, dep.version); err != nil {
				return err
			}
		}
	}

	// Handle dry-run mode
	if addDryRun {
		for _, dep := range toAdd {
			fmt.Fprintf(cmd.OutOrStdout(), "Would add %s@%s to MODULE.bazel\n", dep.name, dep.version)
		}
		return nil
	}

	specs := make([]module.DepSpec, 0, len(toAdd))
	for _, dep := range toAdd {
		specs = append(specs, module.DepSpec{Name: dep.name, Version: dep.version, Dev: addDev})
	}
	newContent, err := module.AppendDeps(content, specs)
	if err != nil {
		return fmt.Errorf("failed to add to MODULE.bazel: %w", err)
	}

	if err := module.WriteFileIfUnchanged(modulePath, content, newContent); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	for _, dep := range toAdd {
		fmt.Fprintf(cmd.OutOrStdout(), "Added %s@%s\n", dep.name, dep.version)
	}

	return nil
}

// validateModule checks if a module and version exist in the registry.
// Returns a helpful error with suggestions if not found.
func validateModule(ctx context.Context, warn io.Writer, reg registry.Registry, name, version string) error {
	meta, err := reg.GetMetadata(ctx, name)
	if err != nil {
		if errors.Is(err, registry.ErrModuleNotFound) {
			return newModuleNotFoundError(ctx, reg, name)
		}
		return fmt.Errorf("failed to validate module %q: %w", name, err)
	}

	// Check if the requested version exists
	versionExists := slices.Contains(meta.Versions, version)

	if !versionExists {
		return newVersionNotFoundError(name, version, meta.Versions)
	}

	if reason, yanked := meta.YankedVersions[version]; yanked {
		if reason != "" {
			fmt.Fprintf(warn, "Warning: %s@%s is yanked: %s\n", name, version, reason)
		} else {
			fmt.Fprintf(warn, "Warning: %s@%s is yanked\n", name, version)
		}
	}

	return nil
}

// newModuleNotFoundError creates a helpful error for module not found.
func newModuleNotFoundError(ctx context.Context, reg registry.Registry, name string) error {
	// Try to get suggestions from the registry
	modules, err := reg.ListModules(ctx)
	if err != nil {
		// If listing is not supported, return simple error with search hint
		return fmt.Errorf("module %q not found in registry\nUse 'bz mod search <query>' to find modules", name)
	}

	suggestions := registry.FindSuggestions(name, modules, 3)
	if len(suggestions) > 0 {
		return &registry.ModuleNotFoundError{
			Module:      name,
			Suggestions: suggestions,
		}
	}

	// No suggestions available
	return fmt.Errorf("module %q not found in registry\nUse 'bz mod search <query>' to find modules", name)
}

// newVersionNotFoundError creates a helpful error for version not found.
func newVersionNotFoundError(name, version string, availableVersions []string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("version %q not found for %s\n", version, name))
	sb.WriteString("Available versions: ")

	// Show up to 5 most recent versions (versions are typically ordered oldest to newest)
	maxVersions := 5
	start := max(len(availableVersions)-maxVersions, 0)

	versionsToShow := make([]string, 0, maxVersions)
	// Reverse order to show newest first
	for i := len(availableVersions) - 1; i >= start; i-- {
		versionsToShow = append(versionsToShow, availableVersions[i])
	}

	sb.WriteString(strings.Join(versionsToShow, ", "))
	if start > 0 {
		sb.WriteString(", ...")
	}

	return errors.New(sb.String())
}

// parseModuleArg splits "module@version" into name and version
func parseModuleArg(arg string) (name, version string) {
	return modulearg.Parse(arg)
}
