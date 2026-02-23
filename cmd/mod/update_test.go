package mod

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/testutil"
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

	updateDryRun = false

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)
	defer updateCmd.SetOut(os.Stdout)

	err := updateCmd.RunE(updateCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	// Should show error for this module
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "error")
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

	// Try to update one that exists and one that doesn't
	err := updateCmd.RunE(updateCmd, []string{"rules_go", "nonexistent"})
	require.NoError(t, err)

	output := buf.String()
	// Should warn about nonexistent
	assert.Contains(t, output, "Warning")
	assert.Contains(t, output, "nonexistent")
	// Should still update rules_go
	assert.Contains(t, output, "rules_go")

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `version = "0.50.1"`)
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
