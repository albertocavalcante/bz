package mod

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/testutil"
	"github.com/albertocavalcante/bz/internal/version"
)

func TestUpdateCmd_UpdatesAllDependencies(t *testing.T) {
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
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.46.0")
	assert.Contains(t, output, "0.50.1")
	assert.Contains(t, output, "rules_python")
	assert.Contains(t, output, "0.31.0")
	assert.Contains(t, output, "1.0.0")

	// Verify MODULE.bazel was updated
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `bazel_dep(name = "rules_go", version = "0.50.1")`)
	assert.Contains(t, string(content), `bazel_dep(name = "rules_python", version = "1.0.0")`)
}

func TestInvalidRegistryVersionErrorRedactsMalformedCredentials(t *testing.T) {
	const secret = "S3CR3T"
	err := registryUpgradeError("rules_go", "https://user:"+secret+"@", errors.New("invalid version"))

	assert.NotContains(t, err.Error(), secret)
	assert.Contains(t, err.Error(), "https://<redacted>")
}

func TestUpdateCmd_UpdatesSpecificDependency(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.46.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.30.0", "1.0.0"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
bazel_dep(name = "rules_python", version = "0.30.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	// Update only rules_go
	err := updateCmd.RunE(updateCmd, []string{"rules_go"})
	require.NoError(t, err)

	// Verify MODULE.bazel
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	// rules_go should be updated
	assert.Contains(t, string(content), `bazel_dep(name = "rules_go", version = "0.50.1")`)
	// rules_python should remain unchanged
	assert.Contains(t, string(content), `bazel_dep(name = "rules_python", version = "0.30.0")`)
}

func TestUpdateCmd_UpdatesMultipleSpecificDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.46.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.30.0", "1.0.0"}},
		"rules_java":   {Versions: []string{"7.0.0", "8.0.0"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
bazel_dep(name = "rules_python", version = "0.30.0")
bazel_dep(name = "rules_java", version = "7.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	// Update rules_go and rules_java, but not rules_python
	err := updateCmd.RunE(updateCmd, []string{"rules_go", "rules_java"})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `bazel_dep(name = "rules_go", version = "0.50.1")`)
	assert.Contains(t, string(content), `bazel_dep(name = "rules_java", version = "8.0.0")`)
	// rules_python should remain unchanged
	assert.Contains(t, string(content), `bazel_dep(name = "rules_python", version = "0.30.0")`)
}

func TestUpdateCmd_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.46.0", "0.50.1"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = true
	defer func() { updateDryRun = false }()

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	// Should show what would be updated
	output := buf.String()
	assert.Contains(t, output, "Would update")
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.46.0")
	assert.Contains(t, output, "0.50.1")

	// File should be unchanged
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `version = "0.46.0"`)
	assert.NotContains(t, string(content), `version = "0.50.1"`)
}

func TestUpdateCmd_AllUpToDate(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.0", "0.50.1"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "All dependencies are up to date")
}

func TestUpdateCmd_SkipsBCRDowngrade(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(fmt.Sprintf("dry-run=%t", dryRun), func(t *testing.T) {
			useRegistry(t, map[string]testutil.TestModule{
				"a": {Versions: []string{"1.2.3.bcr.9"}},
			})
			const content = "module(name = \"m\")\nbazel_dep(name = \"a\", version = \"1.2.3.bcr.10\")\n"
			dir := includeWorkspace(t, map[string]string{"MODULE.bazel": content})
			updateDryRun = dryRun
			var out bytes.Buffer
			updateCmd.SetOut(&out)
			t.Cleanup(func() {
				updateDryRun = false
				updateCmd.SetOut(os.Stdout)
			})

			require.NoError(t, updateCmd.RunE(updateCmd, nil))

			assert.Equal(t, content, readRel(t, dir, "MODULE.bazel"))
			assert.NotContains(t, out.String(), "Updated a")
			assert.NotContains(t, out.String(), "Would update a")
			assert.Contains(t, out.String(), "All dependencies are up to date")
		})
	}
}

func TestModCommandsAgreeOnBazelUpgrades(t *testing.T) {
	useRegistry(t, map[string]testutil.TestModule{
		"a": {Versions: []string{"1.2.3.bcr.10", "2.0.0+build", "1.2.3.bcr.8"}, YankedVersions: map[string]string{"2.0.0": "withdrawn"}},
		"b": {Versions: []string{"1.0.0-rc.10", "1.0.0-rc.9"}},
		"c": {Versions: []string{"1.1.0+other"}},
		"d": {Versions: []string{"1.2.0"}},
	})
	const before = `module(name = "m")
bazel_dep(name = "a", version = "1.2.3.bcr.9")
bazel_dep(name = "b", version = "1.0.0-rc.2")
bazel_dep(name = "c", version = "1.1.0+previous")
bazel_dep(name = "d", version = "1.2")
`
	const after = `module(name = "m")
bazel_dep(name = "a", version = "1.2.3.bcr.10")
bazel_dep(name = "b", version = "1.0.0-rc.10")
bazel_dep(name = "c", version = "1.1.0+previous")
bazel_dep(name = "d", version = "1.2.0")
`
	dir := includeWorkspace(t, map[string]string{"MODULE.bazel": before})

	var out bytes.Buffer
	outdatedCmd.SetOut(&out)
	updateCmd.SetOut(&out)
	outdatedJSON = true
	t.Cleanup(func() {
		outdatedCmd.SetOut(os.Stdout)
		updateCmd.SetOut(os.Stdout)
		outdatedJSON = false
		updateDryRun = false
	})

	require.NoError(t, outdatedCmd.RunE(outdatedCmd, nil))
	var result struct {
		Outdated int           `json:"outdated"`
		Deps     []OutdatedDep `json:"dependencies"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.Equal(t, 3, result.Outdated)
	assert.Equal(t, []OutdatedDep{
		{Name: "a", Current: "1.2.3.bcr.9", Latest: "1.2.3.bcr.10", UpdateType: version.NonSemVer},
		{Name: "b", Current: "1.0.0-rc.2", Latest: "1.0.0-rc.10", UpdateType: version.Patch},
		{Name: "c", Current: "1.1.0+previous", Latest: "1.1.0+other", UpdateType: version.None},
		{Name: "d", Current: "1.2", Latest: "1.2.0", UpdateType: version.NonSemVer},
	}, result.Deps)

	out.Reset()
	updateDryRun = true
	require.NoError(t, updateCmd.RunE(updateCmd, nil))
	assert.Equal(t, before, readRel(t, dir, "MODULE.bazel"))
	assert.Equal(t, "Would update a: 1.2.3.bcr.9 -> 1.2.3.bcr.10 (non-semver)\n"+
		"Would update b: 1.0.0-rc.2 -> 1.0.0-rc.10 (patch)\n"+
		"Would update d: 1.2 -> 1.2.0 (non-semver)\n", out.String())

	out.Reset()
	updateDryRun = false
	require.NoError(t, updateCmd.RunE(updateCmd, nil))
	assert.Equal(t, after, readRel(t, dir, "MODULE.bazel"))
	assert.Equal(t, "Updated a: 1.2.3.bcr.9 -> 1.2.3.bcr.10 (non-semver)\n"+
		"Updated b: 1.0.0-rc.2 -> 1.0.0-rc.10 (patch)\n"+
		"Updated d: 1.2 -> 1.2.0 (non-semver)\n", out.String())
}

func TestUpdateCmd_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "No dependencies found")
}

func TestUpdateCmd_ModuleNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	// Try to update a module that doesn't exist in MODULE.bazel
	err := updateCmd.RunE(updateCmd, []string{"nonexistent"})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Warning")
	assert.Contains(t, output, "nonexistent")
	assert.Contains(t, output, "not found")
}

func TestUpdateCmd_ModuleNotInRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry WITHOUT rules_go
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_python": {Versions: []string{"1.0.0"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	t.Cleanup(func() {
		updateDryRun = false
		updateCmd.SetOut(os.Stdout)
	})
	for _, dryRun := range []bool{true, false} {
		buf.Reset()
		updateDryRun = dryRun
		require.NoError(t, updateCmd.RunE(updateCmd, nil))
		assert.Contains(t, buf.String(), "rules_go: error fetching metadata")
		assert.Contains(t, buf.String(), "No verified dependency updates found.")
		assert.NotContains(t, buf.String(), "All dependencies are up to date")
		assert.Equal(t, moduleContent, readRel(t, tmpDir, "MODULE.bazel"))
	}
}

func TestUpdateCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	updateDryRun = false

	err := updateCmd.RunE(updateCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestUpdateCmd_SkipsPrereleases(t *testing.T) {
	tmpDir := t.TempDir()

	// Registry has a prerelease as the highest version
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.46.0", "0.50.1", "0.51.0-rc1"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	// Should update to 0.50.1, not 0.51.0-rc1
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `version = "0.50.1"`)
	assert.NotContains(t, string(content), `0.51.0-rc1`)
}

func TestUpdateCmd_PreservesOtherContent(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.46.0", "0.50.1"}},
	})

	moduleContent := `# This is a comment
module(name = "test_module", version = "1.0.0")

# Dependencies
bazel_dep(name = "rules_go", version = "0.46.0")

# Register toolchains
register_toolchains("@rules_go//go:toolchain")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	// Comments and other content should be preserved
	assert.Contains(t, string(content), "# This is a comment")
	assert.Contains(t, string(content), "# Dependencies")
	assert.Contains(t, string(content), "register_toolchains")
	// Version should be updated
	assert.Contains(t, string(content), `version = "0.50.1"`)
}

func TestUpdateCmd_PreservesDevDependencyFlag(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"gazelle": {Versions: []string{"0.36.0", "0.38.0"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "gazelle", version = "0.36.0", dev_dependency = True)
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	// Version should be updated but dev_dependency flag preserved
	assert.Contains(t, string(content), `version = "0.38.0"`)
	assert.Contains(t, string(content), "dev_dependency = True")
}

func TestUpdateCmd_MixedFoundAndNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.46.0", "0.50.1"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
bazel_dep(name = "missing_from_registry", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	t.Cleanup(func() {
		updateDryRun = false
		updateCmd.SetOut(os.Stdout)
	})
	for _, dryRun := range []bool{true, false} {
		buf.Reset()
		updateDryRun = dryRun
		require.NoError(t, updateCmd.RunE(updateCmd, []string{"rules_go", "nonexistent", "missing_from_registry"}))
		assert.Contains(t, buf.String(), "Warning: nonexistent not found")
		assert.Contains(t, buf.String(), "missing_from_registry: error fetching metadata")
		if dryRun {
			assert.Contains(t, buf.String(), "Would update rules_go")
			assert.Equal(t, moduleContent, readRel(t, tmpDir, "MODULE.bazel"))
		} else {
			assert.Contains(t, buf.String(), "Updated rules_go")
			assert.Contains(t, readRel(t, tmpDir, "MODULE.bazel"), `version = "0.50.1"`)
		}
	}
}

func TestUpdateCmd_DryRunSpecificModule(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.46.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.30.0", "1.0.0"}},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
bazel_dep(name = "rules_python", version = "0.30.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = true
	defer func() { updateDryRun = false }()

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	// Dry run for specific module
	err := updateCmd.RunE(updateCmd, []string{"rules_go"})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Would update")
	assert.Contains(t, output, "rules_go")
	// rules_python should not be mentioned since we only requested rules_go
	assert.NotContains(t, output, "rules_python")

	// File should be unchanged
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `version = "0.46.0"`)
	assert.Contains(t, string(content), `version = "0.30.0"`)
}

func TestUpdateCmd_MultilineBaselDep(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.46.0", "0.50.1"}},
	})

	// bazel_dep with version on the same line but multiline format
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0", repo_name = "io_bazel_rules_go")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	// Version should be updated and repo_name preserved
	assert.Contains(t, string(content), `version = "0.50.1"`)
	assert.Contains(t, string(content), `repo_name = "io_bazel_rules_go"`)
}

func TestUpdateCmd_ShowsUpdateType(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.46.0", "0.50.1"}}, // minor update
		"rules_python": {Versions: []string{"0.30.0", "1.0.0"}},  // major update
		"rules_java":   {Versions: []string{"7.0.0", "7.0.1"}},   // patch update
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
bazel_dep(name = "rules_python", version = "0.30.0")
bazel_dep(name = "rules_java", version = "7.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	// Should show update types in output
	assert.Contains(t, output, "minor")
	assert.Contains(t, output, "major")
	assert.Contains(t, output, "patch")
}

func TestUpdateCmd_YankedVersionsSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create registry with yanked version using the testutil
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions:       []string{"0.46.0", "0.50.0", "0.50.1"},
			YankedVersions: map[string]string{"0.50.1": "security issue"},
		},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	// Should update to 0.50.0 (not 0.50.1 which is yanked)
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `version = "0.50.0"`)
	assert.NotContains(t, string(content), `version = "0.50.1"`)
}

// TestUpdateCmd_RefusesInvalidRegistryVersion: a version string from the
// registry must never reach MODULE.bazel; the run is all-or-nothing.
func TestUpdateCmd_RefusesInvalidRegistryVersion(t *testing.T) {
	tmpDir := t.TempDir()
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"a": {Versions: []string{"1.0.0"}},
		"b": {Versions: []string{"1.0.0", "2.0.0"}},
	})
	meta := `{"versions": ["9.9.9\", repo_name = \"evil", "1.0.0"]}`
	require.NoError(t, os.WriteFile(filepath.Join(registryDir, "modules", "a", "metadata.json"), []byte(meta), 0o644))

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "b", version = "1.0.0")
bazel_dep(name = "a", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))
	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()
	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	want := `cannot update "a": registry returned invalid module version "9.9.9\", repo_name = \"evil": ` +
		`must match RELEASE[-PRERELEASE][+BUILD] using only A-Z, a-z, 0-9, '.', '-' and '+'` + "\n" +
		fmt.Sprintf("  registry: %q\n", registryDir) +
		"  note: nothing was written to MODULE.bazel\n" +
		"  help: check the registry's metadata for \"a\", or update the other modules by name: bz mod update <module>"
	for _, dryRun := range []bool{true, false} {
		updateDryRun = dryRun
		require.EqualError(t, updateCmd.RunE(updateCmd, nil), want)
		assert.Empty(t, buf.String())
		assert.Equal(t, moduleContent, readRel(t, tmpDir, "MODULE.bazel"))
	}
}

func TestUpdateCmd_RefusesVersionUnsupportedByModuleParser(t *testing.T) {
	useRegistry(t, map[string]testutil.TestModule{
		"a": {Versions: []string{"2.0.0"}},
		"b": {Versions: []string{"snapshot", "2.0.0"}},
	})
	const content = `module(name = "m")
bazel_dep(name = "a", version = "1.0.0")
bazel_dep(name = "b", version = "1.0.0")
`
	dir := includeWorkspace(t, map[string]string{"MODULE.bazel": content})
	var out bytes.Buffer
	updateCmd.SetOut(&out)
	t.Cleanup(func() {
		updateDryRun = false
		updateCmd.SetOut(os.Stdout)
	})
	for _, dryRun := range []bool{true, false} {
		updateDryRun = dryRun
		err := updateCmd.RunE(updateCmd, nil)
		require.ErrorIs(t, err, version.ErrUnsupportedBazelVersion)
		assert.ErrorContains(t, err, `cannot update "b": bz MODULE.bazel parser does not support this Bazel version: "snapshot"`)
		assert.ErrorContains(t, err, "bz mod update <module>")
		assert.Empty(t, out.String())
		assert.Equal(t, content, readRel(t, dir, "MODULE.bazel"))
	}
}

func TestUpdateCmd_DryRunSkipsUneditableDuplicate(t *testing.T) {
	useRegistry(t, map[string]testutil.TestModule{
		"a": {Versions: []string{"1.0.0", "1.1.0"}},
		"c": {Versions: []string{"1.0.0", "1.1.0"}},
	})
	files := map[string]string{
		"MODULE.bazel": "module(name = \"m\")\ninclude(\"//d:x.MODULE.bazel\")\n",
		"d/x.MODULE.bazel": `bazel_dep(name = "a", version = "1.0.0")
bazel_dep(name = "a")
bazel_dep(name = "c", version = "1.0.0", repo_name = "first_c")
bazel_dep(name = "c", version = "", repo_name = "second_c")
`,
	}
	dir := includeWorkspace(t, files)
	var out bytes.Buffer
	updateCmd.SetOut(&out)
	t.Cleanup(func() {
		updateDryRun = false
		updateCmd.SetOut(os.Stdout)
	})

	updateDryRun = true
	require.NoError(t, updateCmd.RunE(updateCmd, nil))
	preview := out.String()
	out.Reset()

	updateDryRun = false
	require.NoError(t, updateCmd.RunE(updateCmd, nil))
	assert.Equal(t, out.String(), preview)
	assert.Equal(t, "Skipping a: no version argument in bazel_dep (version is managed elsewhere) [d/x.MODULE.bazel]\n"+
		"Skipping c: no version argument in bazel_dep (version is managed elsewhere) [d/x.MODULE.bazel]\n", preview)
	for name, want := range files {
		assert.Equal(t, want, readRel(t, dir, name), name)
	}
}

func TestUpdateCmd_RefusesToRewriteUnverifiedDuplicateVersions(t *testing.T) {
	tests := []string{"1.2.3.bcr.11", "1.2.3.bcr.10+build"}
	for _, current := range tests {
		t.Run(current, func(t *testing.T) {
			useRegistry(t, map[string]testutil.TestModule{
				"a": {Versions: []string{"1.2.3.bcr.10"}},
				"b": {Versions: []string{"2.0.0"}},
			})
			content := fmt.Sprintf(`module(name = "m")
bazel_dep(name = "b", version = "1.0.0")
bazel_dep(name = "a", version = "1.2.3.bcr.9", repo_name = "first_a")
bazel_dep(name = "a", version = "%s", repo_name = "second_a")
`, current)
			dir := includeWorkspace(t, map[string]string{"MODULE.bazel": content})
			var out bytes.Buffer
			updateCmd.SetOut(&out)
			t.Cleanup(func() {
				updateDryRun = false
				updateCmd.SetOut(os.Stdout)
			})
			for _, dryRun := range []bool{true, false} {
				updateDryRun = dryRun
				err := updateCmd.RunE(updateCmd, nil)
				require.ErrorContains(t, err, "failed to update a: MODULE.bazel:4")
				assert.ErrorContains(t, err, fmt.Sprintf("has version %q, expected %q", current, "1.2.3.bcr.9"))
				assert.Empty(t, out.String())
				assert.Equal(t, content, readRel(t, dir, "MODULE.bazel"))
			}
		})
	}
}
