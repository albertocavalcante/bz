package mod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestAddCmd_CompletesModuleNames(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.49.0", "0.50.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.34.0", "0.35.0"}},
		"gazelle":      {Versions: []string{"0.37.0", "0.38.0"}},
		"rules_java":   {Versions: []string{"7.0.0", "7.1.0"}},
	})

	// Save and restore the registry flag
	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	// Check that ValidArgsFunction is set
	require.NotNil(t, addCmd.ValidArgsFunction, "addCmd should have ValidArgsFunction set")

	// Test completion for partial module name
	completions, directive := addCmd.ValidArgsFunction(addCmd, []string{}, "rules_")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Contains(t, completions, "rules_go", "should complete rules_go")
	assert.Contains(t, completions, "rules_python", "should complete rules_python")
	assert.NotContains(t, completions, "gazelle", "should not complete gazelle when prefix is rules_")
}

func TestAddCmd_CompletesVersionsAfterAt(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.49.0", "0.50.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.34.0", "0.35.0"}},
		"gazelle":      {Versions: []string{"0.37.0", "0.38.0"}},
		"rules_java":   {Versions: []string{"7.0.0", "7.1.0"}},
	})

	// Save and restore the registry flag
	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	require.NotNil(t, addCmd.ValidArgsFunction, "addCmd should have ValidArgsFunction set")

	// Test completion for versions after @
	completions, directive := addCmd.ValidArgsFunction(addCmd, []string{}, "rules_go@")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Contains(t, completions, "rules_go@0.50.1", "should complete latest version")
	assert.Contains(t, completions, "rules_go@0.50.0", "should complete older versions")
	assert.Contains(t, completions, "rules_go@0.49.0", "should complete oldest version")
}

func TestAddCmd_CompletesVersionsWithPartialVersion(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.49.0", "0.50.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.34.0", "0.35.0"}},
		"gazelle":      {Versions: []string{"0.37.0", "0.38.0"}},
		"rules_java":   {Versions: []string{"7.0.0", "7.1.0"}},
	})

	// Save and restore the registry flag
	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	require.NotNil(t, addCmd.ValidArgsFunction, "addCmd should have ValidArgsFunction set")

	// Test completion for partial version
	completions, directive := addCmd.ValidArgsFunction(addCmd, []string{}, "rules_go@0.50")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Contains(t, completions, "rules_go@0.50.1", "should complete 0.50.1")
	assert.Contains(t, completions, "rules_go@0.50.0", "should complete 0.50.0")
	assert.NotContains(t, completions, "rules_go@0.49.0", "should not complete 0.49.0 when prefix is 0.50")
}

func TestInfoCmd_CompletesModuleNames(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.49.0", "0.50.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.34.0", "0.35.0"}},
		"gazelle":      {Versions: []string{"0.37.0", "0.38.0"}},
		"rules_java":   {Versions: []string{"7.0.0", "7.1.0"}},
	})

	// Save and restore the registry flag
	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	// Check that ValidArgsFunction is set
	require.NotNil(t, infoCmd.ValidArgsFunction, "infoCmd should have ValidArgsFunction set")

	// Test completion for partial module name
	completions, directive := infoCmd.ValidArgsFunction(infoCmd, []string{}, "gaz")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Contains(t, completions, "gazelle", "should complete gazelle")
	assert.NotContains(t, completions, "rules_go", "should not complete rules_go when prefix is gaz")
}

func TestInfoCmd_CompletesVersionsAfterAt(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.49.0", "0.50.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.34.0", "0.35.0"}},
		"gazelle":      {Versions: []string{"0.37.0", "0.38.0"}},
		"rules_java":   {Versions: []string{"7.0.0", "7.1.0"}},
	})

	// Save and restore the registry flag
	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	require.NotNil(t, infoCmd.ValidArgsFunction, "infoCmd should have ValidArgsFunction set")

	// Test completion for versions after @
	completions, directive := infoCmd.ValidArgsFunction(infoCmd, []string{}, "gazelle@")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Contains(t, completions, "gazelle@0.38.0", "should complete latest version")
	assert.Contains(t, completions, "gazelle@0.37.0", "should complete older version")
}

func TestRmCmd_CompletesInstalledModules(t *testing.T) {
	// Create a temporary MODULE.bazel with dependencies
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	// Change to the temp directory
	t.Chdir(tmpDir)

	// Check that ValidArgsFunction is set
	require.NotNil(t, rmCmd.ValidArgsFunction, "rmCmd should have ValidArgsFunction set")

	// Test completion for installed modules
	completions, directive := rmCmd.ValidArgsFunction(rmCmd, []string{}, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Contains(t, completions, "rules_go", "should complete rules_go")
	assert.Contains(t, completions, "rules_python", "should complete rules_python")
	assert.Contains(t, completions, "gazelle", "should complete gazelle")
}

func TestRmCmd_CompletesWithPrefix(t *testing.T) {
	// Create a temporary MODULE.bazel with dependencies
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	require.NotNil(t, rmCmd.ValidArgsFunction, "rmCmd should have ValidArgsFunction set")

	// Test completion with prefix
	completions, directive := rmCmd.ValidArgsFunction(rmCmd, []string{}, "rules_")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Contains(t, completions, "rules_go", "should complete rules_go")
	assert.Contains(t, completions, "rules_python", "should complete rules_python")
	assert.NotContains(t, completions, "gazelle", "should not complete gazelle when prefix is rules_")
}

func TestRmCmd_ExcludesAlreadyRemovedModules(t *testing.T) {
	// Create a temporary MODULE.bazel with dependencies
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	require.NotNil(t, rmCmd.ValidArgsFunction, "rmCmd should have ValidArgsFunction set")

	// Test completion excluding already specified modules
	completions, directive := rmCmd.ValidArgsFunction(rmCmd, []string{"rules_go"}, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.NotContains(t, completions, "rules_go", "should not complete already specified module")
	assert.Contains(t, completions, "rules_python", "should complete rules_python")
	assert.Contains(t, completions, "gazelle", "should complete gazelle")
}

func TestAddCmd_NoCompletionsWithoutRegistry(t *testing.T) {
	// Use a non-existent registry
	oldRegistry := registryFlag
	registryFlag = "/nonexistent/path"
	defer func() { registryFlag = oldRegistry }()

	require.NotNil(t, addCmd.ValidArgsFunction, "addCmd should have ValidArgsFunction set")

	// Should return empty completions gracefully, not error
	completions, directive := addCmd.ValidArgsFunction(addCmd, []string{}, "rules_")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Empty(t, completions, "should return empty completions when registry is unavailable")
}

func TestRmCmd_NoCompletionsWithoutModuleFile(t *testing.T) {
	// Use a directory without MODULE.bazel
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	require.NotNil(t, rmCmd.ValidArgsFunction, "rmCmd should have ValidArgsFunction set")

	// Should return empty completions gracefully
	completions, directive := rmCmd.ValidArgsFunction(rmCmd, []string{}, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Empty(t, completions, "should return empty completions when no MODULE.bazel")
}

func TestAddCmd_CompletesEmptyPrefix(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.49.0", "0.50.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.34.0", "0.35.0"}},
		"gazelle":      {Versions: []string{"0.37.0", "0.38.0"}},
		"rules_java":   {Versions: []string{"7.0.0", "7.1.0"}},
	})

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	require.NotNil(t, addCmd.ValidArgsFunction, "addCmd should have ValidArgsFunction set")

	// Test completion with empty prefix (should return all modules)
	completions, directive := addCmd.ValidArgsFunction(addCmd, []string{}, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Contains(t, completions, "rules_go", "should complete rules_go")
	assert.Contains(t, completions, "rules_python", "should complete rules_python")
	assert.Contains(t, completions, "gazelle", "should complete gazelle")
	assert.Contains(t, completions, "rules_java", "should complete rules_java")
}

func TestInfoCmd_NoCompletionAfterFirstArg(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.49.0", "0.50.0", "0.50.1"}},
		"rules_python": {Versions: []string{"0.34.0", "0.35.0"}},
		"gazelle":      {Versions: []string{"0.37.0", "0.38.0"}},
		"rules_java":   {Versions: []string{"7.0.0", "7.1.0"}},
	})

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	require.NotNil(t, infoCmd.ValidArgsFunction, "infoCmd should have ValidArgsFunction set")

	// infoCmd takes exactly 1 arg, so no completions after first
	completions, directive := infoCmd.ValidArgsFunction(infoCmd, []string{"rules_go"}, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive, "should disable file completion")
	assert.Empty(t, completions, "should return empty completions after first arg for info command")
}
