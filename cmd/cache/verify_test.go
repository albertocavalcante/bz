package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/registry"
)

// resetVerifyCmd resets the verify command state for testing.
func resetVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           verifyCmd.Use,
		Short:         verifyCmd.Short,
		Long:          verifyCmd.Long,
		RunE:          runVerify,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().BoolVar(&verifyJSON, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&verifyCacheDir, "cache-dir", "", "Cache directory (default: ~/.cache/bz)")
	return cmd
}

func TestVerifyCmd_Help(t *testing.T) {
	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "verify")
	assert.Contains(t, output, "cache")
}

func TestVerifyCmd_CompleteCache(t *testing.T) {
	// Create a temporary project with MODULE.bazel
	projectDir := t.TempDir()
	moduleBazelPath := filepath.Join(projectDir, "MODULE.bazel")
	moduleBazelContent := `module(name = "test_project", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(moduleBazelPath, []byte(moduleBazelContent), 0o644))

	// Create cache with the required module
	cacheDir := t.TempDir()
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 100)

	// Change to project directory
	t.Chdir(projectDir)

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go@0.50.1")
	assert.Contains(t, output, "COMPLETE")
}

func TestVerifyCmd_IncompleteCache(t *testing.T) {
	// Create a temporary project with MODULE.bazel
	projectDir := t.TempDir()
	moduleBazelPath := filepath.Join(projectDir, "MODULE.bazel")
	moduleBazelContent := `module(name = "test_project", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "gazelle", version = "0.38.0")
`
	require.NoError(t, os.WriteFile(moduleBazelPath, []byte(moduleBazelContent), 0o644))

	// Create cache with only one of the required modules
	cacheDir := t.TempDir()
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 100)
	// gazelle is missing!

	// Change to project directory
	t.Chdir(projectDir)

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err := cmd.Execute()
	assert.Error(t, err) // Should return error for incomplete cache

	output := stdout.String()
	assert.Contains(t, output, "rules_go@0.50.1")
	assert.Contains(t, output, "gazelle@0.38.0")
	assert.Contains(t, output, "missing")
	assert.Contains(t, output, "INCOMPLETE")
}

func TestVerifyCmd_JSONOutput(t *testing.T) {
	// Create a temporary project with MODULE.bazel
	projectDir := t.TempDir()
	moduleBazelPath := filepath.Join(projectDir, "MODULE.bazel")
	moduleBazelContent := `module(name = "test_project", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(moduleBazelPath, []byte(moduleBazelContent), 0o644))

	// Create cache with the required module
	cacheDir := t.TempDir()
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 100)

	// Change to project directory
	t.Chdir(projectDir)

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--json"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Parse JSON output
	var result verifyResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.True(t, result.Complete)
	assert.Equal(t, 1, result.Cached)
	assert.Equal(t, 0, result.Missing)
}

func TestVerifyCmd_JSONOutputIncomplete(t *testing.T) {
	// Create a temporary project with MODULE.bazel
	projectDir := t.TempDir()
	moduleBazelPath := filepath.Join(projectDir, "MODULE.bazel")
	moduleBazelContent := `module(name = "test_project", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "gazelle", version = "0.38.0")
`
	require.NoError(t, os.WriteFile(moduleBazelPath, []byte(moduleBazelContent), 0o644))

	// Create cache with only one of the required modules
	cacheDir := t.TempDir()
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 100)

	// Change to project directory
	t.Chdir(projectDir)

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--json"})

	err := cmd.Execute()
	assert.Error(t, err)

	// Parse JSON output
	var result verifyResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.False(t, result.Complete)
	assert.Equal(t, 1, result.Cached)
	assert.Equal(t, 1, result.Missing)
	assert.Contains(t, result.MissingModules, "gazelle@0.38.0")
}

func TestVerifyCmd_MalformedCachedModuleIsIncomplete(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "MODULE.bazel"), []byte(`module(name = "test_project", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
`), 0o644))

	cacheDir := t.TempDir()
	modulePath := filepath.Join(cacheDir, "modules", "rules_go", "0.50.1", "MODULE.bazel")
	require.NoError(t, os.MkdirAll(filepath.Dir(modulePath), 0o755))
	require.NoError(t, os.WriteFile(modulePath, []byte("bazel_dep("), 0o644))

	t.Chdir(projectDir)
	verifyJSON = false
	cmd := resetVerifyCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--json"})

	err := cmd.Execute()
	require.Error(t, err)
	var result verifyResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
	assert.False(t, result.Complete)
	assert.Equal(t, []string{"rules_go@0.50.1"}, result.MissingModules)
}

func TestReadCachedModuleBazel_RejectsModulesSymlink(t *testing.T) {
	cacheDir := t.TempDir()
	externalModules := t.TempDir()
	modulePath := filepath.Join(externalModules, "rules_go", "0.50.1", "MODULE.bazel")
	require.NoError(t, os.MkdirAll(filepath.Dir(modulePath), 0o755))
	require.NoError(t, os.WriteFile(modulePath, []byte(`module(name = "rules_go", version = "0.50.1")`), 0o644))
	if err := os.Symlink(externalModules, filepath.Join(cacheDir, "modules")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := registry.NewCacheRegistry(cacheDir).GetModuleBazel(context.Background(), "rules_go", "0.50.1")
	assert.Error(t, err)
}

func TestVerifyCmd_WalksCyclesVersionsAndDevDependencies(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "MODULE.bazel"), []byte(`module(name = "project", version = "1.0.0")
bazel_dep(name = "a", version = "1.0.0")
`), 0o644))
	cacheDir := t.TempDir()
	writeCachedModule := func(name, version, content string) {
		t.Helper()
		path := filepath.Join(cacheDir, "modules", name, version, "MODULE.bazel")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	writeCachedModule("a", "1.0.0", `module(name = "a", version = "1.0.0")
bazel_dep(name = "b", version = "1.0.0")
bazel_dep(name = "b", version = "2.0.0")
bazel_dep(name = "dev", version = "1.0.0", dev_dependency = True)
`)
	writeCachedModule("b", "1.0.0", `module(name = "b", version = "1.0.0")
bazel_dep(name = "a", version = "1.0.0")
`)
	writeCachedModule("b", "2.0.0", `module(name = "b", version = "2.0.0")
`)
	writeCachedModule("dev", "1.0.0", `module(name = "dev", version = "1.0.0")`)

	t.Chdir(projectDir)
	verifyJSON = false
	cmd := resetVerifyCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--json"})

	require.NoError(t, cmd.Execute())
	var result verifyResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
	assert.True(t, result.Complete)
	assert.ElementsMatch(t, []string{"a@1.0.0", "b@1.0.0", "b@2.0.0", "dev@1.0.0"}, result.CachedModules)
}

func TestPrintVerifyText_QuotesInvalidCoordinate(t *testing.T) {
	var stdout bytes.Buffer
	printVerifyText(&stdout, []moduleStatus{{name: "\x1b[31mspoof", version: "1.0.0"}})
	assert.NotContains(t, stdout.String(), "\x1b")
	assert.Contains(t, stdout.String(), `"\x1b[31mspoof"@"1.0.0"`)
}

func TestVerifyCmd_NoMODULEBazel(t *testing.T) {
	// Create a temporary directory without MODULE.bazel
	projectDir := t.TempDir()

	// Change to project directory
	t.Chdir(projectDir)

	cacheDir := t.TempDir()

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err := cmd.Execute()
	assert.Error(t, err) // Should error because no MODULE.bazel found
}
