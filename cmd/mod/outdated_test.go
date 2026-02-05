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
	"github.com/albertocavalcante/bz/internal/version"
)

func TestOutdatedCmd_AllUpToDate(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with rules_go having only 0.50.1
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.0", "0.50.1"}},
	})

	// Setup MODULE.bazel with current latest version
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Set registry flag
	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	outdatedJSON = false

	var buf bytes.Buffer
	outdatedCmd.SetOut(&buf)
	defer outdatedCmd.SetOut(os.Stdout)

	err := outdatedCmd.RunE(outdatedCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.50.1")
	assert.Contains(t, output, "All dependencies are up to date!")
}

func TestOutdatedCmd_HasUpdates(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with newer versions available
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.46.0", "0.48.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.30.0", "0.31.0", "1.0.0"}},
	})

	// Setup MODULE.bazel with older versions
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
bazel_dep(name = "rules_python", version = "0.31.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	outdatedJSON = false

	var buf bytes.Buffer
	outdatedCmd.SetOut(&buf)
	defer outdatedCmd.SetOut(os.Stdout)

	err := outdatedCmd.RunE(outdatedCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	// Should show updates
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.46.0")
	assert.Contains(t, output, "0.50.1")
	assert.Contains(t, output, "minor")

	assert.Contains(t, output, "rules_python")
	assert.Contains(t, output, "0.31.0")
	assert.Contains(t, output, "1.0.0")
	assert.Contains(t, output, "major")

	// Should NOT say all up to date
	assert.NotContains(t, output, "All dependencies are up to date!")
}

func TestOutdatedCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.46.0", "0.50.1"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	outdatedJSON = true
	defer func() { outdatedJSON = false }()

	var buf bytes.Buffer
	outdatedCmd.SetOut(&buf)
	defer outdatedCmd.SetOut(os.Stdout)

	err := outdatedCmd.RunE(outdatedCmd, []string{})
	require.NoError(t, err)

	// Parse JSON output
	var result struct {
		Total    int `json:"total"`
		Outdated int `json:"outdated"`
		Deps     []struct {
			Name       string `json:"name"`
			Current    string `json:"current"`
			Latest     string `json:"latest"`
			UpdateType int    `json:"update_type"`
		} `json:"dependencies"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.Outdated)
	assert.Len(t, result.Deps, 1)
	assert.Equal(t, "rules_go", result.Deps[0].Name)
	assert.Equal(t, "0.46.0", result.Deps[0].Current)
	assert.Equal(t, "0.50.1", result.Deps[0].Latest)
}

func TestOutdatedCmd_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	outdatedJSON = false

	var buf bytes.Buffer
	outdatedCmd.SetOut(&buf)
	defer outdatedCmd.SetOut(os.Stdout)

	err := outdatedCmd.RunE(outdatedCmd, []string{})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "No dependencies found")
}

func TestOutdatedCmd_ModuleNotInRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry WITHOUT rules_go
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_python": {Versions: []string{"0.31.0"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	outdatedJSON = false

	var buf bytes.Buffer
	outdatedCmd.SetOut(&buf)
	defer outdatedCmd.SetOut(os.Stdout)

	err := outdatedCmd.RunE(outdatedCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "error")
}

func TestOutdatedCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	outdatedJSON = false

	err := outdatedCmd.RunE(outdatedCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestOutdatedDep_UpdateTypes(t *testing.T) {
	tests := []struct {
		current    string
		latest     string
		wantType   version.UpdateType
		wantString string
	}{
		{"1.0.0", "1.0.0", version.None, "-"},
		{"1.0.0", "1.0.1", version.Patch, "patch"},
		{"1.0.0", "1.1.0", version.Minor, "minor"},
		{"1.0.0", "2.0.0", version.Major, "major"},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_to_"+tt.latest, func(t *testing.T) {
			got := version.ClassifyUpdate(tt.current, tt.latest)
			assert.Equal(t, tt.wantType, got)
			assert.Equal(t, tt.wantString, got.String())
		})
	}
}
