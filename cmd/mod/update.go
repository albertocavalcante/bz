package mod

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/bzconfig"
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
	Short: "Update dependencies to newer registry versions",
	Long: `Update bazel_dep entries in MODULE.bazel to newer registry versions.

Without arguments, updates all dependencies. With module names specified,
updates only those modules.

Uses Bazel version order to choose the highest non-yanked stable version.
If none is available, considers the highest non-yanked prerelease.

Examples:
  bz mod update                    # update all dependencies
  bz mod update rules_go           # update specific module
  bz mod update rules_go gazelle   # update multiple modules
  bz mod update --dry-run          # preview changes without modifying
  bz mod update --registry=https://my-registry.example.com`,
	RunE:              runUpdate,
	ValidArgsFunction: completeInstalledModules,
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
	file       string // workspace-relative file that declares the dependency
}

// inFile names the declaring file for dependencies outside the root file.
func (u updateInfo) inFile(root string) string {
	if u.file == "" || u.file == root {
		return ""
	}
	return " [" + u.file + "]"
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

	ctx := cmdutil.CommandContext(cmd)
	ws, snapshot, err := module.OpenWorkspaceForUpdate(ctx, modulePath)
	if err != nil {
		return err
	}
	defer ws.Close()
	f := ws.File
	deps := f.UniqueDeps()

	// A module declared in several files must not be updated in just one.
	for _, dep := range deps {
		name := dep.Name.String()
		if len(args) == 0 || slices.Contains(args, name) {
			if err := ws.CheckDeclaredOnce(name); err != nil {
				return err
			}
		}
	}

	out := cmd.OutOrStdout()

	if len(deps) == 0 {
		fmt.Fprintln(out, "No dependencies found in MODULE.bazel")
		return nil
	}

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
		for _, dep := range deps {
			existingDeps[dep.Name.String()] = true
		}
		for name := range targetModules {
			if !existingDeps[name] {
				fmt.Fprintf(out, "Warning: %s not found in MODULE.bazel\n", name)
			}
		}
	}

	rootFile := filepath.Base(modulePath)
	versionlessReported := make(map[string]bool)
	reportVersionless := func(u updateInfo) {
		if versionlessReported[u.name] {
			return
		}
		versionlessReported[u.name] = true
		fmt.Fprintf(out, "Skipping %s: no version argument in bazel_dep (version is managed elsewhere)%s\n",
			u.name, u.inFile(rootFile))
	}

	// Check each dependency for updates
	var updates []updateInfo
	for _, dep := range deps {
		name := dep.Name.String()
		current := dep.Version.String()

		// Skip if not in target list (when specific modules are requested)
		if len(targetModules) > 0 && !targetModules[name] {
			continue
		}

		info := updateInfo{
			name:    name,
			current: current,
			file:    ws.RelPath(f.OriginOf(dep)),
		}
		if current == "" {
			reportVersionless(info)
			continue
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

		info.latest, info.updateType, err = version.RegistryUpgrade(current, meta.Versions, meta.YankedVersions)
		if err != nil {
			registryURL, resolveErr := resolvedRegistryURL()
			if resolveErr != nil {
				return resolveErr
			}
			return registryUpgradeError(name, registryURL, err)
		}
		updates = append(updates, info)
	}

	// Only versions classified as newer may be previewed or applied.
	var pendingUpdates []updateInfo
	var skippedUnverified bool
	for _, u := range updates {
		if u.err != nil {
			skippedUnverified = true
			fmt.Fprintf(out, "%s: error fetching metadata: %v\n", u.name, u.err)
			continue
		}
		switch {
		case u.updateType.IsUpgrade():
			pendingUpdates = append(pendingUpdates, u)
		case u.updateType == version.Unknown:
			skippedUnverified = true
			fmt.Fprintf(out, "Skipping %s: cannot verify that %s is newer than %s%s\n",
				u.name, u.latest, u.current, u.inFile(rootFile))
		}
	}

	if len(pendingUpdates) == 0 {
		if skippedUnverified {
			fmt.Fprintln(out, "No verified dependency updates found.")
		} else {
			fmt.Fprintln(out, "All dependencies are up to date!")
		}
		return nil
	}

	requests := make([]module.VersionEdit, 0, len(pendingUpdates))
	for _, u := range pendingUpdates {
		requests = append(requests, module.VersionEdit{Rel: u.file, Name: u.name, Current: u.current, Version: u.latest})
	}
	plan, err := module.PlanVersionEdits(snapshot, requests)
	if err != nil {
		return err
	}

	for _, i := range plan.Versionless {
		reportVersionless(pendingUpdates[i])
	}
	if len(plan.Applied) == 0 && skippedUnverified {
		fmt.Fprintln(out, "No verified dependency updates found.")
	}

	action := "Updated"
	if updateDryRun {
		action = "Would update"
		// Preserve declaration order in previews and file order in apply output.
		slices.Sort(plan.Applied)
	} else if len(plan.Files) > 0 {
		if err := ws.VerifyUnchanged(ctx, snapshot); err != nil {
			return err
		}
		if err := ws.WriteAll(plan.Files); err != nil {
			return err
		}
	}

	for _, i := range plan.Applied {
		u := pendingUpdates[i]
		fmt.Fprintf(out, "%s %s: %s -> %s (%s)%s\n",
			action, u.name, u.current, u.latest, u.updateType.String(), u.inFile(rootFile))
	}

	return nil
}

// registryUpgradeError explains why the chosen version cannot be applied.
// The registry location is shown without credentials.
func registryUpgradeError(name, registryURL string, cause error) error {
	source := "registry returned "
	help := fmt.Sprintf("check the registry's metadata for %q, or ", name)
	if errors.Is(cause, version.ErrUnsupportedBazelVersion) {
		source, help = "", ""
	}
	shown := bzconfig.RedactURL(registryURL)
	return fmt.Errorf("cannot update %q: %s%w\n  registry: %q\n"+
		"  note: nothing was written to MODULE.bazel\n"+
		"  help: %supdate the other modules by name: bz mod update <module>",
		name, source, cause, shown, help)
}
