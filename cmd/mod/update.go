package mod

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

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
	line       int
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

	f, err := module.Load(modulePath)
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

		info := updateInfo{
			name:    name,
			current: current,
			line:    dep.Pos.Line,
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

	// Apply updates to the file
	if err := applyUpdates(modulePath, pendingUpdates); err != nil {
		return err
	}

	// Print what was updated
	for _, u := range pendingUpdates {
		fmt.Fprintf(out, "Updated %s: %s -> %s (%s)\n",
			u.name, u.current, u.latest, u.updateType.String())
	}

	return nil
}

// applyUpdates modifies the MODULE.bazel file with the version updates.
func applyUpdates(modulePath string, updates []updateInfo) error {
	// Build a map of line -> new version for quick lookup
	lineUpdates := make(map[int]updateInfo)
	for _, u := range updates {
		lineUpdates[u.line] = u
	}

	// Read file and update lines
	file, err := os.Open(modulePath)
	if err != nil {
		return fmt.Errorf("failed to open MODULE.bazel: %w", err)
	}
	defer file.Close()

	// Regex to match version in bazel_dep
	// Matches: version = "x.y.z" or version="x.y.z"
	versionRegex := regexp.MustCompile(`(version\s*=\s*")([^"]+)(")`)

	var newLines []string
	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()

		if update, ok := lineUpdates[lineNum]; ok {
			// Replace the version in this line
			newLine := versionRegex.ReplaceAllString(line, "${1}"+update.latest+"${3}")
			newLines = append(newLines, newLine)
		} else {
			newLines = append(newLines, line)
		}
		lineNum++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	// Write updated content
	newContent := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		newContent += "\n"
	}

	if err := os.WriteFile(modulePath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	return nil
}
