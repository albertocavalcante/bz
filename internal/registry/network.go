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

func withNetworkPolicy[T any](
	r *NetworkAwareRegistry,
	offline func() (T, error),
	preferOffline func() (T, error),
	online func() (T, error),
) (T, error) {
	if r.isOffline() {
		return offline()
	}
	if r.isPreferOffline() {
		return preferOffline()
	}
	return online()
}

func (r *NetworkAwareRegistry) sourceGetter(reg Registry) SourceGetter {
	sg, ok := reg.(SourceGetter)
	if !ok {
		return nil
	}
	return sg
}

// GetMetadata fetches the metadata for a module, respecting network mode.
func (r *NetworkAwareRegistry) GetMetadata(ctx context.Context, module string) (*Metadata, error) {
	return withNetworkPolicy(
		r,
		func() (*Metadata, error) {
			if r.cache == nil {
				return nil, NewOfflineCacheMissError(module, r.inner.String())
			}
			meta, err := r.cache.GetMetadata(ctx, module)
			if err != nil {
				return nil, NewOfflineCacheMissError(module, r.inner.String())
			}
			return meta, nil
		},
		func() (*Metadata, error) {
			if r.cache != nil {
				meta, err := r.cache.GetMetadata(ctx, module)
				if err == nil {
					return meta, nil
				}
			}
			return r.inner.GetMetadata(ctx, module)
		},
		func() (*Metadata, error) {
			return r.inner.GetMetadata(ctx, module)
		},
	)
}

// GetModuleBazel fetches the MODULE.bazel content, respecting network mode.
func (r *NetworkAwareRegistry) GetModuleBazel(ctx context.Context, module, version string) ([]byte, error) {
	return withNetworkPolicy(
		r,
		func() ([]byte, error) {
			if r.cache == nil {
				return nil, NewOfflineVersionMissError(module, version, r.inner.String())
			}
			content, err := r.cache.GetModuleBazel(ctx, module, version)
			if err != nil {
				return nil, NewOfflineVersionMissError(module, version, r.inner.String())
			}
			return content, nil
		},
		func() ([]byte, error) {
			if r.cache != nil {
				content, err := r.cache.GetModuleBazel(ctx, module, version)
				if err == nil {
					return content, nil
				}
			}
			return r.inner.GetModuleBazel(ctx, module, version)
		},
		func() ([]byte, error) {
			return r.inner.GetModuleBazel(ctx, module, version)
		},
	)
}

// ListModules lists all modules, respecting network mode.
func (r *NetworkAwareRegistry) ListModules(ctx context.Context) ([]string, error) {
	return withNetworkPolicy(
		r,
		func() ([]string, error) {
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
		},
		func() ([]string, error) {
			if r.cache != nil {
				modules, err := r.cache.ListModules(ctx)
				if err == nil && len(modules) > 0 {
					return modules, nil
				}
			}
			return r.inner.ListModules(ctx)
		},
		func() ([]string, error) {
			return r.inner.ListModules(ctx)
		},
	)
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
	return withNetworkPolicy(
		r,
		func() ([]byte, error) {
			if r.cache == nil {
				return nil, NewOfflineVersionMissError(module, version, r.inner.String())
			}
			sg := r.sourceGetter(r.cache)
			if sg == nil {
				return nil, nil
			}
			data, err := sg.GetSource(ctx, module, version)
			if err != nil {
				return nil, NewOfflineVersionMissError(module, version, r.inner.String())
			}
			return data, nil
		},
		func() ([]byte, error) {
			if r.cache != nil {
				if sg := r.sourceGetter(r.cache); sg != nil {
					data, err := sg.GetSource(ctx, module, version)
					if err == nil {
						return data, nil
					}
				}
			}
			if sg := r.sourceGetter(r.inner); sg != nil {
				return sg.GetSource(ctx, module, version)
			}
			return nil, nil
		},
		func() ([]byte, error) {
			if sg := r.sourceGetter(r.inner); sg != nil {
				return sg.GetSource(ctx, module, version)
			}
			return nil, nil
		},
	)
}

// Verify NetworkAwareRegistry implements Registry.
var _ Registry = (*NetworkAwareRegistry)(nil)

// Verify NetworkAwareRegistry implements SourceGetter.
var _ SourceGetter = (*NetworkAwareRegistry)(nil)
