package registry

import (
	"fmt"
	"strings"
	"time"

	"github.com/albertocavalcante/go-bcr"
)

// Well-known registry URLs.
const (
	// DefaultBCR is the default Bazel Central Registry URL.
	DefaultBCR = "https://bcr.bazel.build"

	// CloudflareMirror is the Cloudflare mirror of BCR.
	CloudflareMirror = "https://bcr.cloudflaremirrors.com"
)

// URL scheme prefixes.
const (
	schemeHTTPS = "https://"
	schemeHTTP  = "http://"
	schemeFile  = "file://"
)

// New creates a Registry from a URL string.
//
// Supported URL formats:
//   - https://example.com/registry - HTTP registry
//   - http://example.com/registry - HTTP registry
//   - file:///path/to/registry - Filesystem registry
//   - /absolute/path - Filesystem registry (implicit file://)
//
// Returns ErrInvalidRegistry for unsupported formats.
func New(url string) (Registry, error) {
	if url == "" {
		return nil, ErrInvalidRegistry
	}

	// HTTPS
	if strings.HasPrefix(url, schemeHTTPS) {
		return NewHTTPRegistry(url), nil
	}

	// HTTP
	if strings.HasPrefix(url, schemeHTTP) {
		return NewHTTPRegistry(url), nil
	}

	// Explicit file:// URL
	if path, ok := strings.CutPrefix(url, schemeFile); ok {
		return NewFileRegistry(path), nil
	}

	// Absolute path (Unix-style)
	if strings.HasPrefix(url, "/") {
		return NewFileRegistry(url), nil
	}

	// Windows absolute paths (C:\... or C:/...)
	if isWindowsAbsolutePath(url) {
		return NewFileRegistry(url), nil
	}

	return nil, fmt.Errorf("%w: unsupported URL format %q", ErrInvalidRegistry, url)
}

// Default returns the default BCR registry.
func Default() Registry {
	return NewHTTPRegistry(DefaultBCR)
}

// MustNew is like New but panics on error.
// Use only for compile-time constant URLs.
func MustNew(url string) Registry {
	reg, err := New(url)
	if err != nil {
		panic(err)
	}
	return reg
}

// Options for creating registries with additional features.
type options struct {
	cacheDir string
	cacheTTL time.Duration
}

// Option configures registry creation.
type Option func(*options)

// WithCacheDir enables caching in the specified directory.
// Only applies to HTTP registries.
func WithCacheDir(dir string) Option {
	return func(o *options) {
		o.cacheDir = dir
	}
}

// WithCacheTTL sets the cache time-to-live duration.
// Only applies when caching is enabled.
// Default: 1 hour
func WithCacheTTL(ttl time.Duration) Option {
	return func(o *options) {
		o.cacheTTL = ttl
	}
}

// NewWithOptions creates a Registry from a URL string with optional configuration.
// Uses go-bcr for the underlying implementation, which provides caching support.
//
// Supported URL formats:
//   - https://example.com/registry - HTTP registry (with optional caching)
//   - http://example.com/registry - HTTP registry (with optional caching)
//   - file:///path/to/registry - Filesystem registry
//   - /absolute/path - Filesystem registry (implicit file://)
//   - C:\path - Filesystem registry (Windows)
//
// Returns ErrInvalidRegistry for unsupported formats.
func NewWithOptions(url string, opts ...Option) (Registry, error) {
	if url == "" {
		return nil, ErrInvalidRegistry
	}

	cfg := &options{
		cacheTTL: time.Hour, // Default TTL
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Try file registry first (via go-bcr)
	if fileReg, ok := bcr.NewFileRegistryFromURL(url); ok {
		return FromBCR(fileReg), nil
	}

	// HTTPS
	if strings.HasPrefix(url, schemeHTTPS) {
		return newHTTPRegistryWithBCR(url, cfg), nil
	}

	// HTTP
	if strings.HasPrefix(url, schemeHTTP) {
		return newHTTPRegistryWithBCR(url, cfg), nil
	}

	return nil, fmt.Errorf("%w: unsupported URL format %q", ErrInvalidRegistry, url)
}

// newHTTPRegistryWithBCR creates an HTTP registry using go-bcr.
func newHTTPRegistryWithBCR(url string, cfg *options) Registry {
	bcrOpts := []bcr.Option{
		bcr.WithBaseURL(url),
	}

	if cfg.cacheDir != "" {
		bcrOpts = append(bcrOpts, bcr.WithCacheDir(cfg.cacheDir))
		if cfg.cacheTTL > 0 {
			bcrOpts = append(bcrOpts, bcr.WithCacheTTL(cfg.cacheTTL))
		}
	}

	client := bcr.New(bcrOpts...)
	return FromBCR(client)
}

// isWindowsAbsolutePath checks if a path is a Windows absolute path.
// Matches patterns like: C:\path, C:/path, D:\path, d:/path
// Does NOT match: C:path (relative), C: (just drive), or UNC paths.
func isWindowsAbsolutePath(path string) bool {
	if len(path) < 3 {
		return false
	}

	// Check for drive letter (A-Z or a-z)
	letter := path[0]
	if !((letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z')) {
		return false
	}

	// Must be followed by colon
	if path[1] != ':' {
		return false
	}

	// Must be followed by path separator (\ or /)
	return path[2] == '\\' || path[2] == '/'
}
