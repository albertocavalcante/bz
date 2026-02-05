// Package testutil provides shared test utilities for the bz project.
package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModule defines the configuration for a test module in a mock registry.
type TestModule struct {
	// Versions is the list of versions available for this module.
	Versions []string

	// Deps maps version -> list of dependencies as "name@version" strings.
	// Dependencies are written as bazel_dep() in the MODULE.bazel file.
	Deps map[string][]string

	// DevDeps maps version -> list of dev dependencies as "name@version" strings.
	// Dev dependencies are written as bazel_dep() with dev_dependency = True.
	DevDeps map[string][]string

	// License is the SPDX license identifier (e.g., "Apache-2.0", "MIT").
	// If set, it will be included in source.json.
	License string

	// YankedVersions maps version -> reason for yanking.
	// If set, these will be included in metadata.json.
	YankedVersions map[string]string
}

// SetupTestRegistry creates a file-based Bazel Central Registry structure
// in a temporary directory for testing purposes.
//
// The registry follows the BCR structure:
//
//	<registry>/
//	  modules/
//	    <module_name>/
//	      metadata.json
//	      <version>/
//	        MODULE.bazel
//	        source.json
//
// Returns the path to the registry root directory.
func SetupTestRegistry(t *testing.T, modules map[string]TestModule) string {
	t.Helper()

	registryDir := t.TempDir()
	modulesDir := filepath.Join(registryDir, "modules")

	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		t.Fatalf("failed to create modules directory: %v", err)
	}

	for name, mod := range modules {
		createModule(t, modulesDir, name, mod)
	}

	return registryDir
}

// createModule creates a single module in the registry.
func createModule(t *testing.T, modulesDir, name string, mod TestModule) {
	t.Helper()

	moduleDir := filepath.Join(modulesDir, name)
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module directory for %s: %v", name, err)
	}

	// Create metadata.json
	createMetadata(t, moduleDir, mod)

	// Create version directories
	for _, ver := range mod.Versions {
		createVersion(t, moduleDir, name, ver, mod)
	}
}

// createMetadata creates the metadata.json file for a module.
func createMetadata(t *testing.T, moduleDir string, mod TestModule) {
	t.Helper()

	metadata := map[string]any{
		"versions": mod.Versions,
	}

	if len(mod.YankedVersions) > 0 {
		metadata["yanked_versions"] = mod.YankedVersions
	}

	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	if err := os.WriteFile(filepath.Join(moduleDir, "metadata.json"), metadataBytes, 0o644); err != nil {
		t.Fatalf("failed to write metadata.json: %v", err)
	}
}

// createVersion creates a version directory with MODULE.bazel and source.json.
func createVersion(t *testing.T, moduleDir, name, version string, mod TestModule) {
	t.Helper()

	verDir := filepath.Join(moduleDir, version)
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		t.Fatalf("failed to create version directory %s: %v", version, err)
	}

	// Create MODULE.bazel
	createModuleBazel(t, verDir, name, version, mod)

	// Create source.json
	createSourceJSON(t, verDir, name, version, mod)
}

// createModuleBazel creates the MODULE.bazel file for a version.
func createModuleBazel(t *testing.T, verDir, name, version string, mod TestModule) {
	t.Helper()

	var content strings.Builder

	content.WriteString(`module(name = "`)
	content.WriteString(name)
	content.WriteString(`", version = "`)
	content.WriteString(version)
	content.WriteString("\")\n")

	// Add regular dependencies
	if deps, ok := mod.Deps[version]; ok {
		for _, dep := range deps {
			depName, depVer := SplitDepArg(dep)
			content.WriteString("\nbazel_dep(name = \"")
			content.WriteString(depName)
			content.WriteString("\", version = \"")
			content.WriteString(depVer)
			content.WriteString("\")")
		}
	}

	// Add dev dependencies
	if devDeps, ok := mod.DevDeps[version]; ok {
		for _, dep := range devDeps {
			depName, depVer := SplitDepArg(dep)
			content.WriteString("\nbazel_dep(name = \"")
			content.WriteString(depName)
			content.WriteString("\", version = \"")
			content.WriteString(depVer)
			content.WriteString("\", dev_dependency = True)")
		}
	}

	content.WriteString("\n")

	if err := os.WriteFile(filepath.Join(verDir, "MODULE.bazel"), []byte(content.String()), 0o644); err != nil {
		t.Fatalf("failed to write MODULE.bazel: %v", err)
	}
}

// createSourceJSON creates the source.json file for a version.
func createSourceJSON(t *testing.T, verDir, name, version string, mod TestModule) {
	t.Helper()

	source := map[string]any{
		"url":       "https://example.com/" + name + "-" + version + ".tar.gz",
		"integrity": "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}

	if mod.License != "" {
		source["license"] = mod.License
	}

	sourceBytes, err := json.MarshalIndent(source, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal source.json: %v", err)
	}

	if err := os.WriteFile(filepath.Join(verDir, "source.json"), sourceBytes, 0o644); err != nil {
		t.Fatalf("failed to write source.json: %v", err)
	}
}

// SplitDepArg splits a dependency string in "name@version" format.
// Returns (name, version). If no @ is present, returns (input, "").
func SplitDepArg(dep string) (name, version string) {
	idx := strings.Index(dep, "@")
	if idx == -1 {
		return dep, ""
	}
	return dep[:idx], dep[idx+1:]
}
