package modsync

import (
	"context"
	"fmt"
	"log/slog"
	"path"

	"github.com/albertocavalcante/bz/internal/config"
	"github.com/albertocavalcante/bz/internal/publisher"
	"github.com/albertocavalcante/bz/internal/registry"
	bzversion "github.com/albertocavalcante/bz/internal/version"
)

// runner executes a single workflow.
type runner struct {
	workflow *config.Workflow
	source   registry.Registry
	dest     publisher.Publisher
	opts     Options
}

// newRunner creates a runner for a workflow.
func newRunner(workflow *config.Workflow, opts Options) (*runner, error) {
	// Create source registry
	source, err := registryFromConfig(workflow.Origin)
	if err != nil {
		return nil, fmt.Errorf("creating source registry: %w", err)
	}

	// Create destination publisher
	dest, err := publisherFromConfig(workflow.Destination)
	if err != nil {
		return nil, fmt.Errorf("creating destination publisher: %w", err)
	}

	return &runner{
		workflow: workflow,
		source:   source,
		dest:     dest,
		opts:     opts,
	}, nil
}

// close releases resources.
func (r *runner) close() {
	if r.dest != nil {
		_ = r.dest.Close()
	}
}

// run executes the workflow.
func (r *runner) run(ctx context.Context) *Result {
	result := &Result{}

	// Get list of modules to sync (from override or workflow)
	modules := r.opts.Modules
	if len(modules) == 0 {
		modules = r.workflow.Modules
	}

	if len(modules) == 0 {
		return result
	}

	// Process each module
	for _, moduleName := range modules {
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, ctx.Err())
			return result
		default:
		}

		err := r.syncModule(ctx, moduleName, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("module %s: %w", moduleName, err))
			// Continue with other modules
		}
	}

	// Finalize destination (for git: commit & push)
	if !r.opts.DryRun && result.ModulesWritten > 0 {
		message := fmt.Sprintf("Sync %d modules from %s", result.ModulesWritten, r.workflow.Name)
		if err := r.dest.Finalize(ctx, message); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("finalize: %w", err))
		}
	}

	return result
}

// syncModule syncs a single module.
func (r *runner) syncModule(ctx context.Context, moduleName string, result *Result) error {
	result.ModulesProcessed++

	// Get metadata from source
	meta, err := r.source.GetMetadata(ctx, moduleName)
	if err != nil {
		return fmt.Errorf("get metadata: %w", err)
	}

	// Filter versions based on workflow.Versions
	versions := r.filterVersions(meta)
	if len(versions) == 0 {
		if r.opts.Verbose {
			slog.Info("no versions match selector", "module", moduleName)
		}
		return nil
	}

	// Check if we should skip yanked versions
	skipYanked := r.hasTransform(config.Transform{Type: "skip_yanked"})

	// Process each version
	for _, version := range versions {
		// Skip yanked versions if transform is set
		if skipYanked {
			if _, yanked := meta.YankedVersions[version]; yanked {
				if r.opts.Verbose {
					slog.Info("skipped yanked version", "module", moduleName, "version", version)
				}
				continue
			}
		}

		written, err := r.syncVersion(ctx, moduleName, version)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s@%s: %w", moduleName, version, err))
			continue
		}

		if written {
			result.ModulesWritten++
		} else {
			result.ModulesSkipped++
		}
	}

	return nil
}

// syncVersion syncs a single module version.
func (r *runner) syncVersion(ctx context.Context, moduleName, version string) (written bool, err error) {
	// Check if already exists in destination
	moduleBazelPath := path.Join(moduleName, version, "MODULE.bazel")
	exists, err := r.dest.Exists(ctx, moduleBazelPath)
	if err != nil {
		return false, fmt.Errorf("check exists: %w", err)
	}

	if exists {
		if r.opts.Verbose {
			slog.Info("skipped existing version", "module", moduleName, "version", version)
		}
		return false, nil
	}

	if r.opts.Verbose {
		slog.Info("syncing version", "module", moduleName, "version", version)
	}

	if r.opts.DryRun {
		slog.Info("would sync version (dry-run)", "module", moduleName, "version", version)
		return true, nil
	}

	// Read MODULE.bazel from source
	moduleBazel, err := r.source.GetModuleBazel(ctx, moduleName, version)
	if err != nil {
		return false, fmt.Errorf("get MODULE.bazel: %w", err)
	}

	// Read source.json from source
	sourceJSON, err := r.getSourceJSON(ctx, moduleName, version)
	if err != nil {
		return false, fmt.Errorf("get source.json: %w", err)
	}

	// Apply transformations to source.json
	if len(r.workflow.Transformations) > 0 && sourceJSON != nil {
		sourceJSON, err = applyTransforms(r.workflow.Transformations, sourceJSON)
		if err != nil {
			return false, fmt.Errorf("apply transforms: %w", err)
		}
	}

	// Write MODULE.bazel to destination
	if err := r.dest.Put(ctx, moduleBazelPath, moduleBazel); err != nil {
		return false, fmt.Errorf("write MODULE.bazel: %w", err)
	}

	// Write source.json to destination (if available)
	if sourceJSON != nil {
		sourceJSONPath := path.Join(moduleName, version, "source.json")
		if err := r.dest.Put(ctx, sourceJSONPath, sourceJSON); err != nil {
			return false, fmt.Errorf("write source.json: %w", err)
		}
	}

	return true, nil
}

// getSourceJSON fetches source.json from the registry.
// This is a helper that handles the fact that the registry interface
// doesn't have a direct GetSource method.
func (r *runner) getSourceJSON(ctx context.Context, moduleName, version string) ([]byte, error) {
	if sourceReg, ok := r.source.(registry.SourceGetter); ok {
		return sourceReg.GetSource(ctx, moduleName, version)
	}

	// source.json is optional and not supported by every registry backend.
	return nil, nil
}

// filterVersions filters versions based on the workflow's version selector.
func (r *runner) filterVersions(meta *registry.Metadata) []string {
	if r.workflow.Versions == nil {
		// Default: latest version only
		latest := meta.LatestVersion()
		if latest == "" {
			return nil
		}
		return []string{latest}
	}

	switch r.workflow.Versions.Type {
	case "latest":
		count := r.workflow.Versions.Count
		if count <= 0 {
			count = 1
		}
		return latestN(meta.Versions, count)

	case "all":
		return meta.Versions

	case "since":
		return filterSince(meta.Versions, r.workflow.Versions.Since)

	case "range":
		return filterRange(meta.Versions, r.workflow.Versions.MinVer, r.workflow.Versions.MaxVer)

	default:
		// Unknown selector, default to latest
		latest := meta.LatestVersion()
		if latest == "" {
			return nil
		}
		return []string{latest}
	}
}

// hasTransform checks if a transform type is in the workflow.
func (r *runner) hasTransform(t config.Transform) bool {
	for _, wt := range r.workflow.Transformations {
		if wt.Type == t.Type {
			return true
		}
	}
	return false
}

// latestN returns the last N elements from a slice.
func latestN(versions []string, n int) []string {
	if len(versions) == 0 {
		return nil
	}
	if n >= len(versions) {
		return versions
	}
	return versions[len(versions)-n:]
}

// filterSince filters versions by date (simplified: assumes versions are sorted).
// In a real implementation, you'd parse the date and compare with version release dates.
func filterSince(versions []string, since string) []string {
	// For now, just return all versions as we don't have release date info
	// A full implementation would need to fetch metadata for each version
	// or have a separate API to get release dates
	return versions
}

// filterRange filters versions by semver range.
func filterRange(versions []string, minVer, maxVer string) []string {
	var filtered []string
	for _, v := range versions {
		if minVer != "" && bzversion.CompareStrings(v, minVer) < 0 {
			continue
		}
		if maxVer != "" && bzversion.CompareStrings(v, maxVer) > 0 {
			continue
		}
		filtered = append(filtered, v)
	}
	return filtered
}

// registryFromConfig creates a registry from config.
func registryFromConfig(cfg *config.Registry) (registry.Registry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("registry configuration is required")
	}

	switch cfg.Type {
	case "http", "https":
		return registry.NewHTTPRegistry(cfg.URL), nil
	case "file":
		return registry.NewFileRegistry(cfg.URL), nil
	default:
		// Try to create from URL
		return registry.New(cfg.URL)
	}
}

// publisherFromConfig creates a publisher from config.
func publisherFromConfig(cfg *config.Registry) (publisher.Publisher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("registry configuration is required")
	}

	switch cfg.Type {
	case "file":
		return publisher.NewFilePublisher(cfg.URL)

	case "git":
		branch := cfg.Branch
		if branch == "" {
			branch = "main"
		}
		return publisher.NewGitPublisher(cfg.URL, branch)

	case "http_put":
		var auth *publisher.Auth
		if cfg.Auth != nil {
			auth = &publisher.Auth{
				Type:        cfg.Auth.Type,
				Username:    cfg.Auth.Username,
				Password:    cfg.Auth.Password,
				Token:       cfg.Auth.TokenValue,
				EnvVar:      cfg.Auth.EnvVar,
				HeaderName:  cfg.Auth.HeaderName,
				HeaderValue: cfg.Auth.HeaderValue,
			}
		}
		return publisher.NewHTTPPublisher(cfg.URL, auth)

	default:
		return nil, fmt.Errorf("unsupported destination type for publishing: %s", cfg.Type)
	}
}
