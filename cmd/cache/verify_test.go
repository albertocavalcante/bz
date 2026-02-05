package cache

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Chdir(oldWd)) }()
	require.NoError(t, os.Chdir(projectDir))

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err = cmd.Execute()
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
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Chdir(oldWd)) }()
	require.NoError(t, os.Chdir(projectDir))

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err = cmd.Execute()
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
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Chdir(oldWd)) }()
	require.NoError(t, os.Chdir(projectDir))

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--json"})

	err = cmd.Execute()
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
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Chdir(oldWd)) }()
	require.NoError(t, os.Chdir(projectDir))

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--json"})

	err = cmd.Execute()
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

func TestVerifyCmd_NoMODULEBazel(t *testing.T) {
	// Create a temporary directory without MODULE.bazel
	projectDir := t.TempDir()

	// Change to project directory
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Chdir(oldWd)) }()
	require.NoError(t, os.Chdir(projectDir))

	cacheDir := t.TempDir()

	verifyJSON = false
	cmd := resetVerifyCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err = cmd.Execute()
	assert.Error(t, err) // Should error because no MODULE.bazel found
}
