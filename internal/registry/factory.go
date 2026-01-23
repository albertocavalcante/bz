package registry

import (
	"fmt"
	"strings"
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
	if strings.HasPrefix(url, schemeFile) {
		path := strings.TrimPrefix(url, schemeFile)
		return NewFileRegistry(path), nil
	}

	// Absolute path (Unix-style)
	if strings.HasPrefix(url, "/") {
		return NewFileRegistry(url), nil
	}

	// TODO: Windows absolute paths (C:\...)

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
