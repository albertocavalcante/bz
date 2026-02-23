package mod

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCmd_WithValidModuleFile(t *testing.T) {
	// Setup: create temp dir with MODULE.bazel
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)
`
	err := os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644)
	require.NoError(t, err)

	// Change to temp dir
	t.Chdir(tmpDir)

	// Reset flags for test isolation
	listJSON = false

	// Execute command
	var stdout bytes.Buffer
	listCmd.SetOut(&stdout)
	listCmd.SetErr(&stdout)

	err = listCmd.RunE(listCmd, []string{})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.50.1")
	assert.Contains(t, output, "rules_python")
	assert.Contains(t, output, "gazelle")
}

func TestListCmd_WithJSONFlag(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
`
	err := os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644)
	require.NoError(t, err)

	t.Chdir(tmpDir)

	// Set JSON flag
	listJSON = true
	defer func() { listJSON = false }()

	var stdout bytes.Buffer
	listCmd.SetOut(&stdout)

	err = listCmd.RunE(listCmd, []string{})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, `"name"`)
	assert.Contains(t, output, `"rules_go"`)
	assert.Contains(t, output, `"version"`)
}

func TestListCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	listJSON = false

	err := listCmd.RunE(listCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestListCmd_EmptyDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	err := os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644)
	require.NoError(t, err)

	t.Chdir(tmpDir)

	listJSON = false

	var stdout bytes.Buffer
	listCmd.SetOut(&stdout)

	err = listCmd.RunE(listCmd, []string{})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "No dependencies")
}
