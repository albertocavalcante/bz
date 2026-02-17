package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/depgraph"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/modulearg"
	"github.com/albertocavalcante/bz/internal/registry"
)

var (
	downloadAll      bool
	downloadJSON     bool
	downloadRegistry string
	downloadCacheDir string
)

var downloadCmd = &cobra.Command{
	Use:   "download [modules...]",
	Short: "Download modules to local cache",
	Long: `Downloads modules and their transitive dependencies to the local cache.

If no modules are specified, downloads all dependencies from MODULE.bazel
in the current directory (or parent directories).

Use --all to download ALL modules from the registry (warning: this can be very large).

Examples:
  bz cache download                    # Cache all deps from MODULE.bazel
  bz cache download rules_go gazelle   # Cache specific modules
  bz cache download rules_go@0.50.1    # Cache specific version
  bz cache download --all              # Cache ALL modules from registry (large!)`,
	RunE:          runDownload,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func configureDownloadCmd() {
	downloadCmd.Flags().BoolVar(&downloadAll, "all", false, "Download ALL modules from registry (warning: large)")
	downloadCmd.Flags().BoolVar(&downloadJSON, "json", false, "Output as JSON")
	downloadCmd.Flags().StringVar(&downloadRegistry, "registry", "", "Registry URL to download from")
	downloadCmd.Flags().StringVar(&downloadCacheDir, "cache-dir", "", "Cache directory (default: ~/.cache/bz)")
	Cmd.AddCommand(downloadCmd)
}

// downloadResult holds the result of a download operation for JSON output.
type downloadResult struct {
	Downloaded int      `json:"downloaded"`
	Failed     int      `json:"failed"`
	Modules    []string `json:"modules"`
	Errors     []string `json:"errors,omitempty"`
	CacheDir   string   `json:"cache_dir"`
}

//nolint:gocyclo // CLI handler coordinates registry resolution, dependency expansion, and download reporting.
func runDownload(cmd *cobra.Command, args []string) error {
	if err := cli.CheckCommandAllowed("download"); err != nil {
		return err
	}

	ctx := cmdutil.CommandContext(cmd)

	out := cmd.OutOrStdout()

	cacheDir, err := resolveCacheDir(downloadCacheDir)
	if err != nil {
		return err
	}

	registryURL := downloadRegistry
	if registryURL == "" {
		registryURL = cli.GetRegistry()
	}
	if registryURL == "" {
		registryURL = registry.DefaultBCR
	}

	// Create registry client
	reg, err := registry.New(registryURL)
	if err != nil {
		return fmt.Errorf("invalid registry: %w", err)
	}

	// Determine what modules to download
	var modulesToDownload []moduleVersion

	if downloadAll {
		// Download all modules from registry
		modules, err := reg.ListModules(ctx)
		if err != nil {
			return fmt.Errorf("failed to list modules from registry: %w", err)
		}

		if !downloadJSON {
			fmt.Fprintf(out, "Downloading ALL %d modules from registry (this may take a while)...\n\n", len(modules))
		}

		for _, name := range modules {
			meta, err := reg.GetMetadata(ctx, name)
			if err != nil {
				continue // Skip modules we can't fetch metadata for
			}
			// Download all versions
			for _, version := range meta.Versions {
				modulesToDownload = append(modulesToDownload, moduleVersion{name: name, version: version})
			}
		}
	} else if len(args) > 0 {
		// Download specified modules
		for _, arg := range args {
			name, version := parseModuleArg(arg)
			if version == "" {
				// No version specified, get latest
				meta, err := reg.GetMetadata(ctx, name)
				if err != nil {
					return fmt.Errorf("failed to get metadata for %s: %w", name, err)
				}
				version = meta.LatestVersion()
				if version == "" {
					return fmt.Errorf("no available versions for %s", name)
				}
			}
			modulesToDownload = append(modulesToDownload, moduleVersion{name: name, version: version})
		}

		// Resolve transitive dependencies
		modulesToDownload = resolveTransitiveDeps(ctx, reg, modulesToDownload)
	} else {
		// No args - read from MODULE.bazel
		f, err := module.FindAndLoad()
		if err != nil {
			return fmt.Errorf("failed to load MODULE.bazel: %w", err)
		}

		for _, dep := range f.Deps {
			modulesToDownload = append(modulesToDownload, moduleVersion{
				name:    dep.Name.String(),
				version: dep.Version.String(),
			})
		}

		if len(modulesToDownload) == 0 {
			fmt.Fprintln(out, "No dependencies found in MODULE.bazel")
			return nil
		}

		// Resolve transitive dependencies
		modulesToDownload = resolveTransitiveDeps(ctx, reg, modulesToDownload)
	}

	// Download modules
	result := downloadResult{
		CacheDir: cacheDir,
	}

	for _, mv := range modulesToDownload {
		err := downloadModule(ctx, reg, cacheDir, mv.name, mv.version)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s@%s: %v", mv.name, mv.version, err))
			if !downloadJSON {
				fmt.Fprintf(out, "  %s %s@%s: %v\n", errorIcon(), mv.name, mv.version, err)
			}
		} else {
			result.Downloaded++
			result.Modules = append(result.Modules, fmt.Sprintf("%s@%s", mv.name, mv.version))
			if !downloadJSON {
				fmt.Fprintf(out, "  %s Downloaded %s@%s\n", successIcon(), mv.name, mv.version)
			}
		}
	}

	if downloadJSON {
		return printDownloadJSON(out, result)
	}

	fmt.Fprintf(out, "\nDownloaded %d module(s) to %s\n", result.Downloaded, cacheDir)
	if result.Failed > 0 {
		return fmt.Errorf("%d module(s) failed to download", result.Failed)
	}

	return nil
}

// moduleVersion represents a module name and version pair.
type moduleVersion struct {
	name    string
	version string
}

// resolveTransitiveDeps resolves all transitive dependencies for the given modules.
func resolveTransitiveDeps(ctx context.Context, reg registry.Registry, initial []moduleVersion) []moduleVersion {
	refs := make([]depgraph.ModuleRef, 0, len(initial))
	for _, mv := range initial {
		refs = append(refs, depgraph.ModuleRef{Name: mv.name, Version: mv.version})
	}

	resolved := depgraph.ResolveTransitive(ctx, reg, refs)
	result := make([]moduleVersion, 0, len(resolved))
	for _, ref := range resolved {
		result = append(result, moduleVersion{name: ref.Name, version: ref.Version})
	}

	return result
}

// downloadModule downloads a single module to the cache.
func downloadModule(ctx context.Context, reg registry.Registry, cacheDir, name, version string) error {
	// Create module directory structure
	moduleDir := filepath.Join(cacheDir, "modules", name)
	versionDir := filepath.Join(moduleDir, version)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Download metadata.json
	meta, err := reg.GetMetadata(ctx, name)
	if err != nil {
		return fmt.Errorf("get metadata: %w", err)
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "metadata.json"), metaBytes, 0o644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	// Download MODULE.bazel
	moduleBazel, err := reg.GetModuleBazel(ctx, name, version)
	if err != nil {
		return fmt.Errorf("get MODULE.bazel: %w", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "MODULE.bazel"), moduleBazel, 0o644); err != nil {
		return fmt.Errorf("write MODULE.bazel: %w", err)
	}

	// Try to download source.json (optional, not all registries have it)
	if sourceReg, ok := reg.(registry.SourceGetter); ok {
		sourceJSON, err := sourceReg.GetSource(ctx, name, version)
		if err == nil {
			_ = os.WriteFile(filepath.Join(versionDir, "source.json"), sourceJSON, 0o644) // non-fatal: optional metadata
		}
	}

	return nil
}

// parseModuleArg splits "module@version" into name and version.
func parseModuleArg(arg string) (name, version string) {
	return modulearg.Parse(arg)
}

func printDownloadJSON(w io.Writer, result downloadResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// successIcon returns a checkmark character.
func successIcon() string {
	return "\u2713"
}

// errorIcon returns an X character.
func errorIcon() string {
	return "\u2717"
}
