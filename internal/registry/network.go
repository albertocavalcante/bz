package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/albertocavalcante/bz/internal/cli"
)

// NetworkError provides helpful error messages for network failures.
// It includes suggestions for how to resolve the issue.
type NetworkError struct {
	// Operation describes what was being attempted.
	Operation string

	// URL is the registry URL that was being accessed.
	URL string

	// Err is the underlying error.
	Err error

	// Suggestions are helpful hints for resolving the issue.
	Suggestions []string
}

// Error returns a short error message for programmatic use.
func (e *NetworkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Operation, e.URL, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Operation, e.URL)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *NetworkError) Unwrap() error {
	return e.Err
}

// UserFriendlyError returns a formatted error message with suggestions.
// This is suitable for display to users in the terminal.
func (e *NetworkError) UserFriendlyError() string {
	var sb strings.Builder

	// Header with error icon
	sb.WriteString(cli.ErrorIcon())
	sb.WriteString(" Cannot reach registry")
	if e.URL != "" {
		sb.WriteString(" (")
		sb.WriteString(e.URL)
		sb.WriteString(")")
	}
	sb.WriteString("\n\n")

	// Context about the error
	if e.Operation != "" {
		sb.WriteString("  ")
		sb.WriteString(e.Operation)
		sb.WriteString("\n\n")
	}

	// Suggestions
	if len(e.Suggestions) > 0 {
		sb.WriteString("  Options:\n")
		for i, suggestion := range e.Suggestions {
			sb.WriteString(fmt.Sprintf("    %d. %s\n", i+1, suggestion))
		}
	}

	return sb.String()
}

// NewOfflineCacheMissError creates a NetworkError for when a module
// is not found in cache during offline mode.
func NewOfflineCacheMissError(module, registryURL string) *NetworkError {
	return &NetworkError{
		Operation: fmt.Sprintf("You are in offline mode but '%s' is not cached.", module),
		URL:       registryURL,
		Err:       ErrModuleNotFound,
		Suggestions: []string{
			fmt.Sprintf("Cache the module first (on a connected machine):\n       bz cache download %s", module),
			fmt.Sprintf("Use a local registry:\n       bz --registry=file:///path/to/registry mod info %s", module),
			fmt.Sprintf("Disable offline mode:\n       bz --prefer-offline mod info %s", module),
		},
	}
}

// NewOfflineVersionMissError creates a NetworkError for when a specific
// version is not found in cache during offline mode.
func NewOfflineVersionMissError(module, version, registryURL string) *NetworkError {
	return &NetworkError{
		Operation: fmt.Sprintf("You are in offline mode but '%s@%s' is not cached.", module, version),
		URL:       registryURL,
		Err:       ErrVersionNotFound,
		Suggestions: []string{
			fmt.Sprintf("Cache the module first (on a connected machine):\n       bz cache download %s@%s", module, version),
			fmt.Sprintf("Use a local registry:\n       bz --registry=file:///path/to/registry mod info %s", module),
			fmt.Sprintf("Disable offline mode:\n       bz --prefer-offline mod info %s", module),
		},
	}
}

// NetworkAwareRegistry wraps a registry and respects network settings.
// It provides different behaviors for offline, prefer-offline, and online modes.
type NetworkAwareRegistry struct {
	// inner is the underlying network registry.
	inner Registry

	// cache is the local cache registry.
	cache Registry

	// opts contains the network mode settings.
	opts *cli.Options
}

// NewNetworkAware creates a network-aware registry that respects network settings.
//
// Parameters:
//   - inner: The underlying network registry (typically HTTP/HTTPS).
//   - cache: A local cache registry (can be nil if no cache is available).
//   - opts: CLI options containing network mode settings (can be nil for defaults).
//
// Network modes:
//   - Offline: Cache only, returns helpful error if cache miss.
//   - PreferOffline: Cache first, falls back to network on cache miss.
//   - Online (default): Network first (existing behavior).
func NewNetworkAware(inner, cache Registry, opts *cli.Options) *NetworkAwareRegistry {
	return &NetworkAwareRegistry{
		inner: inner,
		cache: cache,
		opts:  opts,
	}
}

// isOffline returns true if offline mode is enabled.
func (r *NetworkAwareRegistry) isOffline() bool {
	return r.opts != nil && r.opts.Offline
}

// isPreferOffline returns true if prefer-offline mode is enabled.
func (r *NetworkAwareRegistry) isPreferOffline() bool {
	return r.opts != nil && r.opts.PreferOffline
}

// GetMetadata fetches the metadata for a module, respecting network mode.
//
// Behavior by mode:
//   - Offline: Cache only, error if miss.
//   - PreferOffline: Cache first, then network.
//   - Online: Network first (existing behavior).
func (r *NetworkAwareRegistry) GetMetadata(ctx context.Context, module string) (*Metadata, error) {
	// Offline mode: cache only
	if r.isOffline() {
		return r.getMetadataFromCacheOnly(ctx, module)
	}

	// Prefer-offline mode: cache first, then network
	if r.isPreferOffline() {
		return r.getMetadataPreferCache(ctx, module)
	}

	// Online mode: use inner registry directly
	return r.inner.GetMetadata(ctx, module)
}

// getMetadataFromCacheOnly attempts to get metadata from cache only.
// Returns a helpful error if not found in cache.
func (r *NetworkAwareRegistry) getMetadataFromCacheOnly(ctx context.Context, module string) (*Metadata, error) {
	if r.cache == nil {
		return nil, NewOfflineCacheMissError(module, r.inner.String())
	}

	meta, err := r.cache.GetMetadata(ctx, module)
	if err != nil {
		return nil, NewOfflineCacheMissError(module, r.inner.String())
	}

	return meta, nil
}

// getMetadataPreferCache tries cache first, falls back to network.
func (r *NetworkAwareRegistry) getMetadataPreferCache(ctx context.Context, module string) (*Metadata, error) {
	// Try cache first if available
	if r.cache != nil {
		meta, err := r.cache.GetMetadata(ctx, module)
		if err == nil {
			return meta, nil
		}
		// Cache miss - fall through to network
	}

	// Fall back to network
	return r.inner.GetMetadata(ctx, module)
}

// GetModuleBazel fetches the MODULE.bazel content, respecting network mode.
func (r *NetworkAwareRegistry) GetModuleBazel(ctx context.Context, module, version string) ([]byte, error) {
	// Offline mode: cache only
	if r.isOffline() {
		return r.getModuleBazelFromCacheOnly(ctx, module, version)
	}

	// Prefer-offline mode: cache first, then network
	if r.isPreferOffline() {
		return r.getModuleBazelPreferCache(ctx, module, version)
	}

	// Online mode: use inner registry directly
	return r.inner.GetModuleBazel(ctx, module, version)
}

// getModuleBazelFromCacheOnly attempts to get MODULE.bazel from cache only.
func (r *NetworkAwareRegistry) getModuleBazelFromCacheOnly(ctx context.Context, module, version string) ([]byte, error) {
	if r.cache == nil {
		return nil, NewOfflineVersionMissError(module, version, r.inner.String())
	}

	content, err := r.cache.GetModuleBazel(ctx, module, version)
	if err != nil {
		return nil, NewOfflineVersionMissError(module, version, r.inner.String())
	}

	return content, nil
}

// getModuleBazelPreferCache tries cache first, falls back to network.
func (r *NetworkAwareRegistry) getModuleBazelPreferCache(ctx context.Context, module, version string) ([]byte, error) {
	// Try cache first if available
	if r.cache != nil {
		content, err := r.cache.GetModuleBazel(ctx, module, version)
		if err == nil {
			return content, nil
		}
		// Cache miss - fall through to network
	}

	// Fall back to network
	return r.inner.GetModuleBazel(ctx, module, version)
}

// ListModules lists all modules, respecting network mode.
func (r *NetworkAwareRegistry) ListModules(ctx context.Context) ([]string, error) {
	// Offline mode: cache only
	if r.isOffline() {
		return r.listModulesFromCacheOnly(ctx)
	}

	// Prefer-offline mode: cache first, then network
	if r.isPreferOffline() {
		return r.listModulesPreferCache(ctx)
	}

	// Online mode: use inner registry directly
	return r.inner.ListModules(ctx)
}

// listModulesFromCacheOnly lists modules from cache only.
func (r *NetworkAwareRegistry) listModulesFromCacheOnly(ctx context.Context) ([]string, error) {
	if r.cache == nil {
		return nil, &NetworkError{
			Operation: "You are in offline mode but no cache is available.",
			URL:       r.inner.String(),
			Err:       ErrListingNotSupported,
			Suggestions: []string{
				"Configure a cache directory in your bz.star config",
				"Use --prefer-offline to allow network fallback",
			},
		}
	}

	return r.cache.ListModules(ctx)
}

// listModulesPreferCache tries cache first, falls back to network.
func (r *NetworkAwareRegistry) listModulesPreferCache(ctx context.Context) ([]string, error) {
	// Try cache first if available
	if r.cache != nil {
		modules, err := r.cache.ListModules(ctx)
		if err == nil && len(modules) > 0 {
			return modules, nil
		}
		// Cache miss or empty - fall through to network
	}

	// Fall back to network
	return r.inner.ListModules(ctx)
}

// Type returns the type of the underlying registry.
func (r *NetworkAwareRegistry) Type() string {
	return r.inner.Type()
}

// String returns a human-readable representation of the registry.
func (r *NetworkAwareRegistry) String() string {
	return r.inner.String()
}

// GetSource fetches source.json, respecting network mode.
// Delegates to the inner registry if it implements SourceGetter.
func (r *NetworkAwareRegistry) GetSource(ctx context.Context, module, version string) ([]byte, error) {
	// Offline mode: cache only
	if r.isOffline() {
		return r.getSourceFromCacheOnly(ctx, module, version)
	}

	// Prefer-offline mode: cache first, then network
	if r.isPreferOffline() {
		return r.getSourcePreferCache(ctx, module, version)
	}

	// Online mode: use inner registry directly
	if sg, ok := r.inner.(SourceGetter); ok {
		return sg.GetSource(ctx, module, version)
	}

	return nil, nil
}

// getSourceFromCacheOnly attempts to get source.json from cache only.
func (r *NetworkAwareRegistry) getSourceFromCacheOnly(ctx context.Context, module, version string) ([]byte, error) {
	if r.cache == nil {
		return nil, NewOfflineVersionMissError(module, version, r.inner.String())
	}

	if sg, ok := r.cache.(SourceGetter); ok {
		data, err := sg.GetSource(ctx, module, version)
		if err != nil {
			return nil, NewOfflineVersionMissError(module, version, r.inner.String())
		}
		return data, nil
	}

	return nil, nil
}

// getSourcePreferCache tries cache first, falls back to network.
func (r *NetworkAwareRegistry) getSourcePreferCache(ctx context.Context, module, version string) ([]byte, error) {
	// Try cache first if available
	if r.cache != nil {
		if sg, ok := r.cache.(SourceGetter); ok {
			data, err := sg.GetSource(ctx, module, version)
			if err == nil {
				return data, nil
			}
			// Cache miss - fall through to network
		}
	}

	// Fall back to network
	if sg, ok := r.inner.(SourceGetter); ok {
		return sg.GetSource(ctx, module, version)
	}

	return nil, nil
}

// Verify NetworkAwareRegistry implements Registry.
var _ Registry = (*NetworkAwareRegistry)(nil)

// Verify NetworkAwareRegistry implements SourceGetter.
var _ SourceGetter = (*NetworkAwareRegistry)(nil)
