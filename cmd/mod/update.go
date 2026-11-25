package mod

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/coord"
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

	ws, err := module.OpenWorkspace(modulePath)
	if err != nil {
		return err
	}
	defer ws.Close()
	f := ws.File
	deps := f.UniqueDeps()

	// Snapshot every declaring file before the slow registry calls, so the
	// write below detects edits made in between.
	snapshot := make(map[string][]byte)
	for _, dep := range deps {
		file := ws.RelPath(f.OriginOf(dep))
		if _, ok := snapshot[file]; ok {
			continue
		}
		content, err := ws.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}
		snapshot[file] = content
	}

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
		for _, dep := range deps {
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
	for _, dep := range deps {
		name := dep.Name.String()
		current := dep.Version.String()

		// Skip if not in target list (when specific modules are requested)
		if len(targetModules) > 0 && !targetModules[name] {
			continue
		}

		if current == "" {
			fmt.Fprintf(out, "Skipping %s: no version argument in bazel_dep (version is managed elsewhere)%s\n", name, updateInfo{file: ws.RelPath(f.OriginOf(dep))}.inFile(filepath.Base(modulePath)))
			continue
		}

		info := updateInfo{
			name:    name,
			current: current,
			file:    ws.RelPath(f.OriginOf(dep)),
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
		} else if err := coord.ValidateVersion(latest); err != nil {
			// All-or-nothing: a registry serving a malformed version is not
			// trusted for the rest of this run either.
			return invalidRegistryVersionError(name, resolvedRegistryURL(), err)
		}

		info.latest = latest
		info.updateType = version.ClassifyUpdate(current, latest)
		updates = append(updates, info)
	}

	// Only versions classified as newer may be previewed or applied.
	var pendingUpdates []updateInfo
	var skippedUnverified bool
	for _, u := range updates {
		if u.err != nil {
			fmt.Fprintf(out, "%s: error fetching metadata: %v\n", u.name, u.err)
			continue
		}
		switch u.updateType {
		case version.Patch, version.Minor, version.Major:
			pendingUpdates = append(pendingUpdates, u)
		case version.Unknown:
			skippedUnverified = true
			fmt.Fprintf(out, "Skipping %s: cannot verify that %s is newer than %s%s\n",
				u.name, u.latest, u.current, u.inFile(filepath.Base(modulePath)))
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

	// Handle dry-run
	if updateDryRun {
		for _, u := range pendingUpdates {
			fmt.Fprintf(out, "Would update %s: %s -> %s (%s)%s\n",
				u.name, u.current, u.latest, u.updateType.String(), u.inFile(filepath.Base(modulePath)))
		}
		return nil
	}

	// Apply updates in memory per file, then write all files together.
	var files []string
	for _, u := range pendingUpdates {
		if !slices.Contains(files, u.file) {
			files = append(files, u.file)
		}
	}
	var edits []module.FileEdit
	var appliedAll []updateInfo
	for _, file := range files {
		var group []updateInfo
		for _, u := range pendingUpdates {
			if u.file == file {
				group = append(group, u)
			}
		}
		newContent, applied, err := applyVersionUpdates(snapshot[file], group, out)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			continue
		}
		edits = append(edits, module.FileEdit{Rel: file, Original: snapshot[file], Updated: newContent})
		appliedAll = append(appliedAll, applied...)
	}

	if len(edits) == 0 {
		return nil
	}

	if err := ws.WriteAll(edits); err != nil {
		return err
	}

	for _, u := range appliedAll {
		fmt.Fprintf(out, "Updated %s: %s -> %s (%s)%s\n",
			u.name, u.current, u.latest, u.updateType.String(), u.inFile(filepath.Base(modulePath)))
	}

	return nil
}

// invalidRegistryVersionError explains a registry version that is not a valid
// Bazel version. The registry location is shown without credentials.
func invalidRegistryVersionError(name, registryURL string, cause error) error {
	shown := registryURL
	if u, err := url.Parse(registryURL); err == nil {
		shown = u.Redacted()
	}
	return fmt.Errorf("cannot update %q: registry returned %w\n  registry: %q\n"+
		"  note: nothing was written to MODULE.bazel\n"+
		"  help: check the registry's metadata for %q, or update the other modules by name: bz mod update <module>",
		name, cause, shown, name)
}

// applyVersionUpdates edits content in memory, one edit per module name
// (SetDepVersion rewrites every statement of that name). Deps without a
// version argument are skipped with a note on out. An edit that changes no
// bytes is an error, and nothing is returned, so the caller writes all or
// nothing. content itself is never modified.
func applyVersionUpdates(content []byte, pending []updateInfo, out io.Writer) ([]byte, []updateInfo, error) {
	newContent := content
	var applied []updateInfo
	seen := make(map[string]bool)
	for _, u := range pending {
		if seen[u.name] {
			continue
		}
		seen[u.name] = true

		next, changed, err := module.SetDepVersion(newContent, u.name, u.latest)
		if err != nil {
			if errors.Is(err, module.ErrNoVersionArgument) {
				fmt.Fprintf(out, "Skipping %s: no version argument in bazel_dep (version is managed elsewhere)%s\n", u.name, u.inFile("MODULE.bazel"))
				continue
			}
			return nil, nil, fmt.Errorf("failed to update %s: %w", u.name, err)
		}
		if !changed {
			return nil, nil, fmt.Errorf("failed to update %s: version %s not found in MODULE.bazel text", u.name, u.current)
		}
		newContent = next
		applied = append(applied, u)
	}
	return newContent, applied, nil
}
