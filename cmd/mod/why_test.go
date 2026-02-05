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

func TestWhyCmd_DirectDependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry: rules_go has no deps
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}},
	})

	// Setup MODULE.bazel with direct dependency on rules_go
	moduleContent := `module(name = "my_project", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	whyJSON = false
	whyAll = false

	var buf bytes.Buffer
	whyCmd.SetOut(&buf)
	defer whyCmd.SetOut(os.Stdout)

	err := whyCmd.RunE(whyCmd, []string{"rules_go"})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "my_project")
	// Direct dependency path: my_project -> rules_go
	assert.Contains(t, output, "direct dependency")
}

func TestWhyCmd_TransitiveDependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry:
	// rules_go@0.50.1 depends on protobuf@21.7
	// protobuf@21.7 has no deps
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps:     map[string][]string{"0.50.1": {"protobuf@21.7"}},
		},
		"protobuf": {Versions: []string{"21.7"}},
	})

	// Setup MODULE.bazel: my_project -> rules_go -> protobuf
	moduleContent := `module(name = "my_project", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	whyJSON = false
	whyAll = false

	var buf bytes.Buffer
	whyCmd.SetOut(&buf)
	defer whyCmd.SetOut(os.Stdout)

	err := whyCmd.RunE(whyCmd, []string{"protobuf"})
	require.NoError(t, err)

	output := buf.String()
	// Should show transitive path: my_project -> rules_go@0.50.1 -> protobuf@21.7
	assert.Contains(t, output, "my_project")
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "protobuf")
}

func TestWhyCmd_MultiplePaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry:
	// rules_go@0.50.1 depends on protobuf@21.7
	// rules_python@0.35.0 depends on protobuf@21.7
	// protobuf@21.7 has no deps
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps:     map[string][]string{"0.50.1": {"protobuf@21.7"}},
		},
		"rules_python": {
			Versions: []string{"0.35.0"},
			Deps:     map[string][]string{"0.35.0": {"protobuf@21.7"}},
		},
		"protobuf": {Versions: []string{"21.7"}},
	})

	// Setup MODULE.bazel: my_project depends on both rules_go and rules_python
	moduleContent := `module(name = "my_project", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	whyJSON = false
	whyAll = true // Show all paths

	var buf bytes.Buffer
	whyCmd.SetOut(&buf)
	defer whyCmd.SetOut(os.Stdout)

	err := whyCmd.RunE(whyCmd, []string{"protobuf"})
	require.NoError(t, err)

	output := buf.String()
	// Should show both paths
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "rules_python")
	assert.Contains(t, output, "protobuf")
}

func TestWhyCmd_NotADependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with rules_go only
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}},
	})

	// Setup MODULE.bazel with rules_go only
	moduleContent := `module(name = "my_project", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	whyJSON = false
	whyAll = false

	var buf bytes.Buffer
	whyCmd.SetOut(&buf)
	defer whyCmd.SetOut(os.Stdout)

	err := whyCmd.RunE(whyCmd, []string{"protobuf"})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "protobuf")
	assert.Contains(t, output, "not a dependency")
}

func TestWhyCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with transitive dep
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps:     map[string][]string{"0.50.1": {"protobuf@21.7"}},
		},
		"protobuf": {Versions: []string{"21.7"}},
	})

	moduleContent := `module(name = "my_project", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	whyJSON = true
	whyAll = false
	defer func() { whyJSON = false }()

	var buf bytes.Buffer
	whyCmd.SetOut(&buf)
	defer whyCmd.SetOut(os.Stdout)

	err := whyCmd.RunE(whyCmd, []string{"protobuf"})
	require.NoError(t, err)

	// Parse JSON output
	var result struct {
		Module string     `json:"module"`
		Found  bool       `json:"found"`
		Paths  [][]string `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "protobuf", result.Module)
	assert.True(t, result.Found)
	assert.NotEmpty(t, result.Paths)
	// First path should be: my_project -> rules_go@0.50.1 -> protobuf@21.7
	assert.Len(t, result.Paths[0], 3)
}

func TestWhyCmd_JSONOutput_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}},
	})

	moduleContent := `module(name = "my_project", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	whyJSON = true
	whyAll = false
	defer func() { whyJSON = false }()

	var buf bytes.Buffer
	whyCmd.SetOut(&buf)
	defer whyCmd.SetOut(os.Stdout)

	err := whyCmd.RunE(whyCmd, []string{"nonexistent"})
	require.NoError(t, err)

	var result struct {
		Module string     `json:"module"`
		Found  bool       `json:"found"`
		Paths  [][]string `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "nonexistent", result.Module)
	assert.False(t, result.Found)
	assert.Empty(t, result.Paths)
}

func TestWhyCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	whyJSON = false

	err := whyCmd.RunE(whyCmd, []string{"protobuf"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestWhyCmd_NoArgs(t *testing.T) {
	tmpDir := t.TempDir()

	moduleContent := `module(name = "my_project", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	whyJSON = false

	err := whyCmd.RunE(whyCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "module name")
}

func TestWhyCmd_DeepTransitiveDependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with a deeper dependency chain:
	// rules_go@0.50.1 -> bazel_skylib@1.5.0 -> platforms@0.0.7
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps:     map[string][]string{"0.50.1": {"bazel_skylib@1.5.0"}},
		},
		"bazel_skylib": {
			Versions: []string{"1.5.0"},
			Deps:     map[string][]string{"1.5.0": {"platforms@0.0.7"}},
		},
		"platforms": {Versions: []string{"0.0.7"}},
	})

	moduleContent := `module(name = "my_project", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	whyJSON = false
	whyAll = false

	var buf bytes.Buffer
	whyCmd.SetOut(&buf)
	defer whyCmd.SetOut(os.Stdout)

	err := whyCmd.RunE(whyCmd, []string{"platforms"})
	require.NoError(t, err)

	output := buf.String()
	// Should show full path: my_project -> rules_go@0.50.1 -> bazel_skylib@1.5.0 -> platforms@0.0.7
	assert.Contains(t, output, "my_project")
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "bazel_skylib")
	assert.Contains(t, output, "platforms")
}
