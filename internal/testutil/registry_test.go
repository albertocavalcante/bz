package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupTestRegistry_BasicModules(t *testing.T) {
	// Test creating a basic registry with just versions
	registryDir := SetupTestRegistry(t, map[string]TestModule{
		"rules_go": {
			Versions: []string{"0.49.0", "0.50.0", "0.50.1"},
		},
		"rules_python": {
			Versions: []string{"0.34.0", "0.35.0"},
		},
	})

	// Verify directory structure
	assert.DirExists(t, registryDir)
	assert.DirExists(t, filepath.Join(registryDir, "modules", "rules_go"))
	assert.DirExists(t, filepath.Join(registryDir, "modules", "rules_python"))

	// Verify metadata.json for rules_go
	metadataPath := filepath.Join(registryDir, "modules", "rules_go", "metadata.json")
	assert.FileExists(t, metadataPath)

	metadataBytes, err := os.ReadFile(metadataPath)
	require.NoError(t, err)

	var metadata map[string]any
	require.NoError(t, json.Unmarshal(metadataBytes, &metadata))

	versions := metadata["versions"].([]any)
	assert.Len(t, versions, 3)

	// Verify version directories and MODULE.bazel files
	for _, ver := range []string{"0.49.0", "0.50.0", "0.50.1"} {
		verDir := filepath.Join(registryDir, "modules", "rules_go", ver)
		assert.DirExists(t, verDir)
		assert.FileExists(t, filepath.Join(verDir, "MODULE.bazel"))

		content, err := os.ReadFile(filepath.Join(verDir, "MODULE.bazel"))
		require.NoError(t, err)
		assert.Contains(t, string(content), `module(name = "rules_go"`)
		assert.Contains(t, string(content), `version = "`+ver+`"`)
	}
}

func TestSetupTestRegistry_WithDependencies(t *testing.T) {
	// Test creating modules with dependencies
	registryDir := SetupTestRegistry(t, map[string]TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps: map[string][]string{
				"0.50.1": {"bazel_skylib@1.5.0", "platforms@0.0.8"},
			},
		},
		"bazel_skylib": {
			Versions: []string{"1.5.0"},
		},
		"platforms": {
			Versions: []string{"0.0.8"},
		},
	})

	// Verify rules_go@0.50.1 has dependencies in MODULE.bazel
	modulePath := filepath.Join(registryDir, "modules", "rules_go", "0.50.1", "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `bazel_dep(name = "bazel_skylib", version = "1.5.0")`)
	assert.Contains(t, string(content), `bazel_dep(name = "platforms", version = "0.0.8")`)
}

func TestSetupTestRegistry_WithDevDependencies(t *testing.T) {
	// Test creating modules with dev dependencies
	registryDir := SetupTestRegistry(t, map[string]TestModule{
		"my_project": {
			Versions: []string{"1.0.0"},
			Deps: map[string][]string{
				"1.0.0": {"rules_go@0.50.1"},
			},
			DevDeps: map[string][]string{
				"1.0.0": {"gazelle@0.38.0"},
			},
		},
		"rules_go": {
			Versions: []string{"0.50.1"},
		},
		"gazelle": {
			Versions: []string{"0.38.0"},
		},
	})

	// Verify dev dependency in MODULE.bazel
	modulePath := filepath.Join(registryDir, "modules", "my_project", "1.0.0", "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `bazel_dep(name = "rules_go", version = "0.50.1")`)
	assert.Contains(t, string(content), `bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)`)
}

func TestSetupTestRegistry_WithLicense(t *testing.T) {
	// Test creating modules with license information
	registryDir := SetupTestRegistry(t, map[string]TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			License:  "Apache-2.0",
		},
		"protobuf": {
			Versions: []string{"21.7"},
			License:  "BSD-3-Clause",
		},
	})

	// Verify source.json with license for rules_go
	sourcePath := filepath.Join(registryDir, "modules", "rules_go", "0.50.1", "source.json")
	assert.FileExists(t, sourcePath)

	sourceBytes, err := os.ReadFile(sourcePath)
	require.NoError(t, err)

	var source map[string]any
	require.NoError(t, json.Unmarshal(sourceBytes, &source))

	assert.Equal(t, "Apache-2.0", source["license"])

	// Verify protobuf license
	sourcePath = filepath.Join(registryDir, "modules", "protobuf", "21.7", "source.json")
	sourceBytes, err = os.ReadFile(sourcePath)
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(sourceBytes, &source))
	assert.Equal(t, "BSD-3-Clause", source["license"])
}

func TestSetupTestRegistry_WithYankedVersions(t *testing.T) {
	// Test creating modules with yanked versions
	registryDir := SetupTestRegistry(t, map[string]TestModule{
		"rules_go": {
			Versions:       []string{"0.49.0", "0.50.0", "0.50.1"},
			YankedVersions: map[string]string{"0.50.0": "security issue"},
		},
	})

	// Verify metadata.json includes yanked_versions
	metadataPath := filepath.Join(registryDir, "modules", "rules_go", "metadata.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	require.NoError(t, err)

	var metadata map[string]any
	require.NoError(t, json.Unmarshal(metadataBytes, &metadata))

	yanked := metadata["yanked_versions"].(map[string]any)
	assert.Equal(t, "security issue", yanked["0.50.0"])
}

func TestSetupTestRegistry_ComplexScenario(t *testing.T) {
	// Test a complex scenario with multiple features
	registryDir := SetupTestRegistry(t, map[string]TestModule{
		"rules_go": {
			Versions: []string{"0.49.0", "0.50.0", "0.50.1"},
			Deps: map[string][]string{
				"0.50.0": {"bazel_skylib@1.4.0", "platforms@0.0.7"},
				"0.50.1": {"bazel_skylib@1.5.0", "platforms@0.0.8"},
			},
			License:        "Apache-2.0",
			YankedVersions: map[string]string{"0.49.0": "buggy"},
		},
		"bazel_skylib": {
			Versions: []string{"1.4.0", "1.5.0"},
			License:  "Apache-2.0",
		},
		"platforms": {
			Versions: []string{"0.0.7", "0.0.8"},
			License:  "Apache-2.0",
		},
	})

	// Verify the complex structure
	assert.DirExists(t, registryDir)

	// Check rules_go@0.50.1 deps
	modulePath := filepath.Join(registryDir, "modules", "rules_go", "0.50.1", "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `bazel_dep(name = "bazel_skylib", version = "1.5.0")`)

	// Check rules_go@0.50.0 deps
	modulePath = filepath.Join(registryDir, "modules", "rules_go", "0.50.0", "MODULE.bazel")
	content, err = os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `bazel_dep(name = "bazel_skylib", version = "1.4.0")`)
}

func TestSetupTestRegistry_EmptyVersionDeps(t *testing.T) {
	// Versions not in Deps map should have no dependencies
	registryDir := SetupTestRegistry(t, map[string]TestModule{
		"rules_go": {
			Versions: []string{"0.49.0", "0.50.1"},
			Deps: map[string][]string{
				"0.50.1": {"platforms@0.0.8"},
				// 0.49.0 not in deps map
			},
		},
		"platforms": {
			Versions: []string{"0.0.8"},
		},
	})

	// 0.49.0 should have no deps
	modulePath := filepath.Join(registryDir, "modules", "rules_go", "0.49.0", "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "bazel_dep")

	// 0.50.1 should have deps
	modulePath = filepath.Join(registryDir, "modules", "rules_go", "0.50.1", "MODULE.bazel")
	content, err = os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "bazel_dep")
}

func TestSetupTestRegistry_SourceJSON_AlwaysCreated(t *testing.T) {
	// source.json should always be created even without license
	registryDir := SetupTestRegistry(t, map[string]TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			// No license specified
		},
	})

	sourcePath := filepath.Join(registryDir, "modules", "rules_go", "0.50.1", "source.json")
	assert.FileExists(t, sourcePath)

	sourceBytes, err := os.ReadFile(sourcePath)
	require.NoError(t, err)

	var source map[string]any
	require.NoError(t, json.Unmarshal(sourceBytes, &source))

	// Should have url and integrity
	assert.NotEmpty(t, source["url"])
	assert.NotEmpty(t, source["integrity"])

	// Should NOT have license field when not specified
	_, hasLicense := source["license"]
	assert.False(t, hasLicense)
}

func TestSplitDepArg(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{"rules_go@0.50.1", "rules_go", "0.50.1"},
		{"bazel_skylib@1.5.0", "bazel_skylib", "1.5.0"},
		{"platforms@0.0.8", "platforms", "0.0.8"},
		{"nodeps", "nodeps", ""},
		{"multiple@at@signs", "multiple", "at@signs"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, version := SplitDepArg(tt.input)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantVersion, version)
		})
	}
}
