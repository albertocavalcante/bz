// Package config provides configuration loading and types for bz.
// It loads and parses bz.star configuration files using the Starlark interpreter.
package config

// Config represents the complete bz configuration from a bz.star file.
type Config struct {
	// Defaults are the CLI defaults
	Defaults *Defaults

	// Workflows are the sync workflows defined
	Workflows []*Workflow
}

// Defaults represents config.defaults() values.
type Defaults struct {
	// Registry is the default registry to use.
	Registry *Registry

	// CacheDir is the directory for caching module data.
	CacheDir string

	// FallbackRegistries is a list of registries to try if the default fails.
	FallbackRegistries []*Registry
}

// Registry represents a registry configuration.
type Registry struct {
	// Type is the registry type: "http", "file", "git", or "http_put".
	Type string

	// URL is the registry URL (for http, git, http_put) or path (for file).
	URL string

	// Branch is the git branch (for git registries).
	Branch string

	// Ref is the git ref (for git registries).
	Ref string

	// Auth is the authentication configuration (for http_put registries).
	Auth *Auth
}

// Auth represents authentication configuration.
type Auth struct {
	// Type is the authentication method: "basic", "bearer_token", or "header".
	Type string

	// Username for basic auth.
	Username string

	// Password for basic auth.
	Password string

	// EnvVar is the environment variable name for bearer token.
	EnvVar string

	// TokenValue is a static bearer token value.
	TokenValue string

	// HeaderName is the custom header name for header auth.
	HeaderName string

	// HeaderValue is the custom header value for header auth.
	HeaderValue string
}

// Workflow represents a sync.workflow().
type Workflow struct {
	// Name is the unique identifier for the workflow.
	Name string

	// Description provides human-readable documentation.
	Description string

	// Origin is the source registry.
	Origin *Registry

	// Destination is the target registry.
	Destination *Registry

	// Modules is the list of module names or patterns to sync.
	Modules []string

	// Versions is the version selection strategy.
	Versions *VersionSelector

	// Transformations is the list of transformations to apply.
	Transformations []*Transform
}

// VersionSelector represents version selection criteria.
type VersionSelector struct {
	// Type is the selector type: "latest", "all", "since", or "range".
	Type string

	// Count is the number of versions for "latest" selector.
	Count int

	// Since is the date string for "since" selector (format: "2024-01-01").
	Since string

	// MinVer is the minimum version for "range" selector.
	MinVer string

	// MaxVer is the maximum version for "range" selector.
	MaxVer string
}

// Transform represents a sync transformation.
type Transform struct {
	// Type is the transformation type: "rewrite_urls", "skip_yanked", or "include_patches".
	Type string

	// Pattern is the regex pattern for URL rewriting.
	Pattern string

	// Replacement is the replacement string for URL rewriting.
	Replacement string

	// Enabled is the flag for include_patches.
	Enabled bool
}
