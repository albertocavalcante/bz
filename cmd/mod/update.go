package mod

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/version"
)

var (
	updateDryRun bool
)

var updateCmd = &cobra.Command{
	Use:   "update [module...]",
	Short: "Update dependencies to latest compatible versions",
	Long: `Update bazel_dep entries in MODULE.bazel to their latest stable versions.

Without arguments, updates all dependencies. With module names specified,
updates only those modules.

Uses LatestStable() from registry metadata, which skips prereleases and
yanked versions.

Examples:
  bz mod update                    # update all dependencies
  bz mod update rules_go           # update specific module
  bz mod update rules_go gazelle   # update multiple modules
  bz mod update --dry-run          # preview changes without modifying
  bz mod update --registry=https://my-registry.example.com`,
	RunE: runUpdate,
}

func configureUpdateCmd() {
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Show what would be updated without making changes")
	Cmd.AddCommand(updateCmd)
}

// updateInfo holds information about a dependency update.
type updateInfo struct {
	name       string
	current    string
	latest     string
	updateType version.UpdateType
	err        error
}

//nolint:gocyclo // CLI command handler with sequential steps
func runUpdate(cmd *cobra.Command, args []string) error {
	// Check if command is disabled
	if err := cli.CheckCommandAllowed("update"); err != nil {
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

	out := cmd.OutOrStdout()

	if len(f.Deps) == 0 {
		fmt.Fprintln(out, "No dependencies found in MODULE.bazel")
		return nil
	}

	ctx := cmdutil.CommandContext(cmd)

	// Create network-aware registry
	reg, err := createNetworkAwareRegistry()
	if err != nil {
		return err
	}

	// Build set of modules to update (empty means all)
	targetModules := make(map[string]bool)
	for _, arg := range args {
		targetModules[arg] = true
	}

	// Warn about modules that don't exist in MODULE.bazel
	if len(targetModules) > 0 {
		existingDeps := make(map[string]bool)
		for _, dep := range f.Deps {
			existingDeps[dep.Name.String()] = true
		}
		for name := range targetModules {
			if !existingDeps[name] {
				fmt.Fprintf(out, "Warning: %s not found in MODULE.bazel\n", name)
			}
		}
	}

	// Check each dependency for updates
	var updates []updateInfo
	for _, dep := range f.Deps {
		name := dep.Name.String()
		current := dep.Version.String()

		// Skip if not in target list (when specific modules are requested)
		if len(targetModules) > 0 && !targetModules[name] {
			continue
		}

		if current == "" {
			fmt.Fprintf(out, "Skipping %s: no version argument in bazel_dep (version is managed elsewhere)\n", name)
			continue
		}

		info := updateInfo{
			name:    name,
			current: current,
		}

		// Fetch metadata from registry
		meta, err := reg.GetMetadata(ctx, name)
		if err != nil {
			info.err = err
			info.latest = current
			info.updateType = version.None
			updates = append(updates, info)
			continue
		}

		// Use LatestVersion which skips prereleases
		latest := meta.LatestVersion()
		if latest == "" {
			latest = current
		}

		info.latest = latest
		info.updateType = version.ClassifyUpdate(current, latest)
		updates = append(updates, info)
	}

	// Filter to only updates that have changes
	var pendingUpdates []updateInfo
	for _, u := range updates {
		if u.updateType != version.None && u.err == nil {
			pendingUpdates = append(pendingUpdates, u)
		}
	}

	// Print errors for modules that couldn't be checked
	for _, u := range updates {
		if u.err != nil {
			fmt.Fprintf(out, "%s: error fetching metadata: %v\n", u.name, u.err)
		}
	}

	if len(pendingUpdates) == 0 {
		fmt.Fprintln(out, "All dependencies are up to date!")
		return nil
	}

	// Handle dry-run
	if updateDryRun {
		for _, u := range pendingUpdates {
			fmt.Fprintf(out, "Would update %s: %s -> %s (%s)\n",
				u.name, u.current, u.latest, u.updateType.String())
		}
		return nil
	}

	// Apply updates in memory, then write once.
	newContent := content
	var applied []updateInfo
	for _, u := range pendingUpdates {
		next, changed, err := module.SetDepVersion(newContent, u.name, u.latest)
		if err != nil {
			if errors.Is(err, module.ErrNoVersionArgument) {
				fmt.Fprintf(out, "Skipping %s: no version argument in bazel_dep (version is managed elsewhere)\n", u.name)
				continue
			}
			return fmt.Errorf("failed to update %s: %w", u.name, err)
		}
		if changed {
			newContent = next
			applied = append(applied, u)
		}
	}

	if len(applied) == 0 {
		return nil
	}

	if err := module.WriteFileAtomic(modulePath, newContent); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	for _, u := range applied {
		fmt.Fprintf(out, "Updated %s: %s -> %s (%s)\n",
			u.name, u.current, u.latest, u.updateType.String())
	}

	return nil
}
