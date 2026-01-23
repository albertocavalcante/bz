// Package registry provides abstractions for BCR-compatible module registries.
//
// A BCR-compatible registry follows this canonical structure:
//
//	modules/
//	├── <module_name>/
//	│   ├── metadata.json
//	│   └── <version>/
//	│       ├── MODULE.bazel
//	│       └── source.json
//
// This package supports multiple backends: filesystem, HTTP, and others.
package registry

import (
	"context"
	"errors"
	"path"
	"strings"
)

// Canonical directory and file names for BCR-compatible registries.
const (
	// ModulesDir is the root directory containing all modules.
	ModulesDir = "modules"

	// MetadataFile is the per-module metadata file.
	MetadataFile = "metadata.json"

	// ModuleBazelFile is the module definition file per version.
	ModuleBazelFile = "MODULE.bazel"

	// SourceFile contains source/archive information per version.
	SourceFile = "source.json"

	// IndexFile is an optional file listing all modules (for registries that support it).
	IndexFile = "index.json"
)

// Registry type identifiers.
const (
	TypeFile  = "file"
	TypeHTTP  = "http"
	TypeHTTPS = "https"
)

// Sentinel errors for registry operations.
var (
	// ErrModuleNotFound indicates the requested module does not exist.
	ErrModuleNotFound = errors.New("module not found")

	// ErrVersionNotFound indicates the requested version does not exist.
	ErrVersionNotFound = errors.New("version not found")

	// ErrListingNotSupported indicates the registry does not support listing modules.
	// Operations like Search will not work, but Get operations will.
	ErrListingNotSupported = errors.New("registry does not support module listing")

	// ErrInvalidRegistry indicates the registry URL/path is malformed.
	ErrInvalidRegistry = errors.New("invalid registry")
)

// Path builders for canonical registry structure.

// ModulePath returns the path to a module directory: modules/<name>
func ModulePath(name string) string {
	return path.Join(ModulesDir, name)
}

// MetadataPath returns the path to a module's metadata: modules/<name>/metadata.json
func MetadataPath(name string) string {
	return path.Join(ModulesDir, name, MetadataFile)
}

// VersionPath returns the path to a version directory: modules/<name>/<version>
func VersionPath(name, version string) string {
	return path.Join(ModulesDir, name, version)
}

// ModuleBazelPath returns the path to MODULE.bazel: modules/<name>/<version>/MODULE.bazel
func ModuleBazelPath(name, version string) string {
	return path.Join(ModulesDir, name, version, ModuleBazelFile)
}

// SourcePath returns the path to source.json: modules/<name>/<version>/source.json
func SourcePath(name, version string) string {
	return path.Join(ModulesDir, name, version, SourceFile)
}

// ModulesIndexPath returns the path to the optional modules index: modules/index.json
func ModulesIndexPath() string {
	return path.Join(ModulesDir, IndexFile)
}

// Registry is the interface for BCR-compatible module registries.
type Registry interface {
	// GetMetadata fetches the metadata.json for a module.
	// Returns ErrModuleNotFound if the module does not exist.
	GetMetadata(ctx context.Context, module string) (*Metadata, error)

	// GetModuleBazel fetches the MODULE.bazel content for a specific version.
	// Returns ErrModuleNotFound or ErrVersionNotFound as appropriate.
	GetModuleBazel(ctx context.Context, module, version string) ([]byte, error)

	// ListModules returns all available module names.
	// Returns ErrListingNotSupported if the registry cannot enumerate modules.
	ListModules(ctx context.Context) ([]string, error)

	// Type returns the registry type identifier (file, http, https).
	Type() string

	// String returns a human-readable representation of the registry.
	String() string
}

// Metadata represents the contents of a module's metadata.json.
type Metadata struct {
	Homepage       string            `json:"homepage,omitempty"`
	Maintainers    []Maintainer      `json:"maintainers,omitempty"`
	Repository     []string          `json:"repository,omitempty"`
	Versions       []string          `json:"versions"`
	YankedVersions map[string]string `json:"yanked_versions,omitempty"`
}

// Maintainer represents a module maintainer.
type Maintainer struct {
	Name         string `json:"name,omitempty"`
	Email        string `json:"email,omitempty"`
	GitHub       string `json:"github,omitempty"`
	GitHubUserID int    `json:"github_user_id,omitempty"`
}

// LatestVersion returns the latest non-yanked, non-prerelease version.
// Falls back to latest non-yanked if all are prereleases.
// Returns empty string if all versions are yanked.
func (m *Metadata) LatestVersion() string {
	// First pass: find latest stable (non-prerelease, non-yanked)
	for i := len(m.Versions) - 1; i >= 0; i-- {
		v := m.Versions[i]
		if _, yanked := m.YankedVersions[v]; yanked {
			continue
		}
		if !isPrerelease(v) {
			return v
		}
	}

	// Second pass: any non-yanked version
	for i := len(m.Versions) - 1; i >= 0; i-- {
		v := m.Versions[i]
		if _, yanked := m.YankedVersions[v]; !yanked {
			return v
		}
	}

	return ""
}

// prereleaseIndicators are common version string patterns indicating prereleases.
var prereleaseIndicators = []string{"-rc", "-alpha", "-beta", "-dev", "-pre"}

// isPrerelease checks if a version string indicates a prerelease.
func isPrerelease(version string) bool {
	for _, indicator := range prereleaseIndicators {
		if strings.Contains(version, indicator) {
			return true
		}
	}
	return false
}
