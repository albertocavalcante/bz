package mod

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestStatsCmd_CountDirectDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with modules (no transitive deps for this test)
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.50.1"}},
		"rules_python": {Versions: []string{"0.35.0"}},
		"gazelle":      {Versions: []string{"0.38.0"}},
	})

	// Setup MODULE.bazel with 3 direct deps
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	statsJSON = false

	var buf bytes.Buffer
	statsCmd.SetOut(&buf)
	defer statsCmd.SetOut(os.Stdout)

	err := statsCmd.RunE(statsCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Direct dependencies:")
	assert.Contains(t, output, "3")
}

func TestStatsCmd_CountTransitiveDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with transitive dependencies:
	// rules_go -> bazel_skylib, platforms
	// rules_python -> bazel_skylib
	// gazelle -> rules_go, bazel_skylib
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps: map[string][]string{
				"0.50.1": {"bazel_skylib@1.5.0", "platforms@0.0.8"},
			},
		},
		"rules_python": {
			Versions: []string{"0.35.0"},
			Deps: map[string][]string{
				"0.35.0": {"bazel_skylib@1.5.0"},
			},
		},
		"gazelle": {
			Versions: []string{"0.38.0"},
			Deps: map[string][]string{
				"0.38.0": {"rules_go@0.50.1", "bazel_skylib@1.5.0"},
			},
		},
		"bazel_skylib": {Versions: []string{"1.5.0"}},
		"platforms":    {Versions: []string{"0.0.8"}},
	})

	// Setup MODULE.bazel with 3 direct deps
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	statsJSON = false

	var buf bytes.Buffer
	statsCmd.SetOut(&buf)
	defer statsCmd.SetOut(os.Stdout)

	err := statsCmd.RunE(statsCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	// Transitive deps: bazel_skylib, platforms, rules_go (via gazelle)
	// Note: rules_go is both direct and transitive via gazelle, counted once as direct
	// So transitive (not counting direct): bazel_skylib, platforms = 2
	assert.Contains(t, output, "Transitive dependencies:")
	assert.Contains(t, output, "2")

	// Total modules: rules_go, rules_python, gazelle, bazel_skylib, platforms = 5
	assert.Contains(t, output, "Total modules:")
	assert.Contains(t, output, "5")
}

func TestStatsCmd_CalculateMaxDepth(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with a chain of dependencies:
	// module_a -> module_b -> module_c -> module_d (depth 4)
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"module_a": {
			Versions: []string{"1.0.0"},
			Deps:     map[string][]string{"1.0.0": {"module_b@1.0.0"}},
		},
		"module_b": {
			Versions: []string{"1.0.0"},
			Deps:     map[string][]string{"1.0.0": {"module_c@1.0.0"}},
		},
		"module_c": {
			Versions: []string{"1.0.0"},
			Deps:     map[string][]string{"1.0.0": {"module_d@1.0.0"}},
		},
		"module_d": {Versions: []string{"1.0.0"}},
	})

	// Setup MODULE.bazel with module_a as direct dep
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "module_a", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	statsJSON = false

	var buf bytes.Buffer
	statsCmd.SetOut(&buf)
	defer statsCmd.SetOut(os.Stdout)

	err := statsCmd.RunE(statsCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	// Max depth: root -> module_a -> module_b -> module_c -> module_d = 4
	assert.Contains(t, output, "Max depth:")
	assert.Contains(t, output, "4")
}

func TestStatsCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with transitive dependencies
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps:     map[string][]string{"0.50.1": {"bazel_skylib@1.5.0"}},
		},
		"bazel_skylib": {Versions: []string{"1.5.0"}},
	})

	// Setup MODULE.bazel with a dev dependency
	// rules_go -> bazel_skylib (transitive)
	// bazel_skylib is also direct (as dev dep)
	// Max depth: root -> rules_go -> bazel_skylib = 2
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "bazel_skylib", version = "1.5.0", dev_dependency = True)
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	statsJSON = true
	defer func() { statsJSON = false }()

	var buf bytes.Buffer
	statsCmd.SetOut(&buf)
	defer statsCmd.SetOut(os.Stdout)

	err := statsCmd.RunE(statsCmd, []string{})
	require.NoError(t, err)

	// Parse JSON output
	var result struct {
		DirectDeps     int `json:"direct_dependencies"`
		TransitiveDeps int `json:"transitive_dependencies"`
		TotalModules   int `json:"total_modules"`
		MaxDepth       int `json:"max_depth"`
		DevDeps        int `json:"dev_dependencies"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, 2, result.DirectDeps)
	assert.Equal(t, 0, result.TransitiveDeps) // bazel_skylib is also direct (as dev dep)
	assert.Equal(t, 2, result.TotalModules)
	assert.Equal(t, 2, result.MaxDepth) // root -> rules_go -> bazel_skylib
	assert.Equal(t, 1, result.DevDeps)
}

func TestStatsCmd_DevDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.50.1"}},
		"rules_python": {Versions: []string{"0.35.0"}},
		"gazelle":      {Versions: []string{"0.38.0"}},
	})

	// Setup MODULE.bazel with 2 dev dependencies out of 3
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0", dev_dependency = True)
bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	statsJSON = false

	var buf bytes.Buffer
	statsCmd.SetOut(&buf)
	defer statsCmd.SetOut(os.Stdout)

	err := statsCmd.RunE(statsCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Dev dependencies:")
	assert.Contains(t, output, "2")
}

func TestStatsCmd_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	statsJSON = false

	var buf bytes.Buffer
	statsCmd.SetOut(&buf)
	defer statsCmd.SetOut(os.Stdout)

	err := statsCmd.RunE(statsCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Direct dependencies:")
	assert.Contains(t, output, "0")
}

func TestStatsCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	statsJSON = false

	err := statsCmd.RunE(statsCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}
