package modsync

import (
	"context"
	"fmt"
	"log/slog"
	"path"

	"github.com/albertocavalcante/bz/internal/config"
	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/publisher"
	"github.com/albertocavalcante/bz/internal/registry"
	bzversion "github.com/albertocavalcante/bz/internal/version"
)

// runner executes a single workflow.
type runner struct {
	workflow *config.Workflow
	source   registry.Registry
	dest     publisher.Publisher
	local    *publisher.FilePublisher
	opts     Options
}

// newRunner creates a runner for a workflow.
func newRunner(workflow *config.Workflow, opts Options) (*runner, error) {
	// Create source registry
	source, err := registryFromConfig(workflow.Origin)
	if err != nil {
		return nil, fmt.Errorf("creating source registry: %w", err)
	}

	// Local mirrors must never publish files that their own reader cannot read.
	if workflow.Destination != nil && workflow.Destination.Type == registry.TypeFile && workflow.Origin != nil && workflow.Origin.Type == registry.TypeFile {
		source = registry.NewCacheRegistry(workflow.Origin.URL)
	}

	// Create destination publisher
	dest, err := publisherFromConfig(workflow.Destination, opts.DryRun)
	if err != nil {
		return nil, fmt.Errorf("creating destination publisher: %w", err)
	}

	local, _ := dest.(*publisher.FilePublisher)
	return &runner{
		workflow: workflow,
		source:   source,
		dest:     dest,
		local:    local,
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

	// Module names come from --modules or the workflow file and become path
	// segments in the destination; refuse invalid ones before any I/O.
	if err := coord.ValidateModuleName(moduleName); err != nil {
		return err
	}

	if r.local != nil && !r.opts.DryRun {
		release, err := r.local.LockRegistry(ctx)
		if err != nil {
			return fmt.Errorf("lock destination: %w", err)
		}
		defer release()
	}

	// Get metadata from source
	meta, err := r.source.GetMetadata(ctx, moduleName)
	if err != nil {
		return fmt.Errorf("get metadata: %w", err)
	}

	// Filter versions based on workflow.Versions
	selectionMeta := normalizedSelectionMetadata(meta)
	versions, err := r.selectVersions(selectionMeta)
	if err != nil {
		return err
	}
	if len(versions) == 0 && r.opts.Verbose {
		slog.Info("no versions match selector", "module", moduleName)
	}

	// Check if we should skip yanked versions
	skipYanked := r.hasTransform(config.Transform{Type: "skip_yanked"})

	var metadata *localMetadata
	if r.local != nil && !r.opts.DryRun {
		metadata, err = readLocalMetadata(r.local, moduleName)
		if err != nil {
			return fmt.Errorf("read destination metadata: %w", err)
		}
	}

	// Process each version
	var available []string
	for _, version := range versions {
		// Versions come from untrusted source metadata; an invalid one fails
		// only that version and nothing is written for it.
		if err := coord.ValidateVersion(version); err != nil {
			// The error quotes the version; do not echo it unquoted.
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", moduleName, err))
			continue
		}

		// Skip yanked versions if transform is set
		if skipYanked {
			if _, yanked := yankReason(meta.YankedVersions, version); yanked {
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

		if metadata != nil {
			available = append(available, version)
		}
		if written {
			result.ModulesWritten++
		} else {
			result.ModulesSkipped++
		}
	}

	if metadata != nil && (len(available) > 0 || metadata.fields != nil) {
		if err := metadata.write(r.local, moduleName, meta, available); err != nil {
			return fmt.Errorf("write destination metadata: %w", err)
		}
	}
	return nil
}

// syncVersion syncs a single module version.
func (r *runner) syncVersion(ctx context.Context, moduleName, version string) (written bool, err error) {
	// Check if already exists in destination
	moduleBazelPath := path.Join(moduleName, version, registry.ModuleBazelFile)
	sourceJSONPath := path.Join(moduleName, version, registry.SourceFile)
	var hasModule, hasSource bool
	if r.local != nil {
		hasModule, err = completeFile(r.local, moduleBazelPath, func(data []byte) bool { return validModule(data, moduleName, version) })
		if err != nil {
			return false, fmt.Errorf("check MODULE.bazel: %w", err)
		}
		hasSource, err = completeFile(r.local, sourceJSONPath, validSource)
		if err != nil {
			return false, fmt.Errorf("check source.json: %w", err)
		}
	} else {
		hasModule, err = r.dest.Exists(ctx, moduleBazelPath)
		if err != nil {
			return false, fmt.Errorf("check exists: %w", err)
		}
	}

	if hasModule && (r.local == nil || hasSource) {
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

	moduleBazel, sourceJSON, err := r.fetchVersion(ctx, moduleName, version, hasModule, hasSource)
	if err != nil {
		return false, err
	}

	// Publish the local version marker after its source description.
	if r.local != nil && !hasSource {
		if err := r.dest.Put(ctx, sourceJSONPath, sourceJSON); err != nil {
			return false, fmt.Errorf("write source.json: %w", err)
		}
	}
	if !hasModule {
		if err := r.dest.Put(ctx, moduleBazelPath, moduleBazel); err != nil {
			return false, fmt.Errorf("write MODULE.bazel: %w", err)
		}
	}
	if r.local == nil && sourceJSON != nil {
		if err := r.dest.Put(ctx, sourceJSONPath, sourceJSON); err != nil {
			return false, fmt.Errorf("write source.json: %w", err)
		}
	}

	return true, nil
}

func (r *runner) fetchVersion(ctx context.Context, moduleName, version string, hasModule, hasSource bool) ([]byte, []byte, error) {
	var err error
	var moduleBazel []byte
	if !hasModule {
		moduleBazel, err = r.source.GetModuleBazel(ctx, moduleName, version)
		if err != nil {
			return nil, nil, fmt.Errorf("get MODULE.bazel: %w", err)
		}
	}

	var sourceJSON []byte
	if !hasSource {
		sourceJSON, err = r.getSourceJSON(ctx, moduleName, version)
		if err != nil {
			return nil, nil, fmt.Errorf("get source.json: %w", err)
		}
		if len(r.workflow.Transformations) > 0 && sourceJSON != nil {
			sourceJSON, err = applyTransforms(r.workflow.Transformations, sourceJSON)
			if err != nil {
				return nil, nil, fmt.Errorf("apply transforms: %w", err)
			}
		}
	}

	if r.local != nil {
		if !hasModule && !validModule(moduleBazel, moduleName, version) {
			return nil, nil, fmt.Errorf("MODULE.bazel is empty or invalid")
		}
		if !hasSource && !validSource(sourceJSON) {
			return nil, nil, fmt.Errorf("source.json is missing or not a JSON object")
		}
	}

	return moduleBazel, sourceJSON, nil
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

func normalizedSelectionMetadata(meta *registry.Metadata) *registry.Metadata {
	// LatestVersion consults exact keys; normalize yanks against origin versions.
	selectionMeta := *meta
	selectionMeta.YankedVersions = make(map[string]string)
	for _, version := range meta.Versions {
		if reason, ok := yankReason(meta.YankedVersions, version); ok {
			selectionMeta.YankedVersions[version] = reason
		}
	}
	return &selectionMeta
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
		return nil // rejected by selectVersions; never select everything

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

// errSinceUnsupported is returned for workflows that use sync.since.
func errSinceUnsupported(workflow string) error {
	return fmt.Errorf("workflow %q: sync.since is not supported yet: release dates are not available from the registry; use sync.latest, sync.range or sync.all", workflow)
}

// selectVersions applies the workflow's version selector, failing for
// selectors that cannot be honored instead of silently selecting nothing.
func (r *runner) selectVersions(meta *registry.Metadata) ([]string, error) {
	if r.workflow.Versions != nil && r.workflow.Versions.Type == "since" {
		return nil, errSinceUnsupported(r.workflow.Name)
	}
	return r.filterVersions(meta), nil
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
func publisherFromConfig(cfg *config.Registry, dryRun bool) (publisher.Publisher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("registry configuration is required")
	}

	switch cfg.Type {
	case "file":
		if dryRun {
			return publisher.OpenFileRegistryPublisher(cfg.URL)
		}
		return publisher.NewFileRegistryPublisher(cfg.URL)

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
				TokenValue:  cfg.Auth.TokenValue,
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
