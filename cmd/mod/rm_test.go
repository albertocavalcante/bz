package mod

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRmCmd_RemovesSingleDependency(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	rmDryRun = false

	err = rmCmd.RunE(rmCmd, []string{"rules_go"})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.NotContains(t, string(content), "rules_go")
	assert.Contains(t, string(content), "rules_python")
	assert.Contains(t, string(content), "test_module")
}

func TestRmCmd_RemovesMultipleDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "rules_java", version = "7.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	rmDryRun = false

	err = rmCmd.RunE(rmCmd, []string{"rules_go", "rules_python"})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.NotContains(t, string(content), "rules_go")
	assert.NotContains(t, string(content), "rules_python")
	assert.Contains(t, string(content), "rules_java")
}

func TestRmCmd_WarnsOnNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	rmDryRun = false

	// Capture output
	var buf bytes.Buffer
	rmCmd.SetOut(&buf)
	defer rmCmd.SetOut(os.Stdout)

	err = rmCmd.RunE(rmCmd, []string{"nonexistent"})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Warning")
	assert.Contains(t, buf.String(), "nonexistent")

	// File should be unchanged
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "rules_go")
}

func TestRmCmd_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	rmDryRun = true
	defer func() { rmDryRun = false }()

	var buf bytes.Buffer
	rmCmd.SetOut(&buf)
	defer rmCmd.SetOut(os.Stdout)

	err = rmCmd.RunE(rmCmd, []string{"rules_go"})
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Would remove")
	assert.Contains(t, buf.String(), "rules_go")

	// File should be unchanged
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "rules_go")
}

func TestRmCmd_RemovesDevDependency(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	rmDryRun = false

	err = rmCmd.RunE(rmCmd, []string{"gazelle"})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.NotContains(t, string(content), "gazelle")
	assert.Contains(t, string(content), "rules_go")
}

func TestRmCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	rmDryRun = false

	err := rmCmd.RunE(rmCmd, []string{"rules_go"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestRmCmd_MixedFoundAndNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	rmDryRun = false

	var buf bytes.Buffer
	rmCmd.SetOut(&buf)
	defer rmCmd.SetOut(os.Stdout)

	// Remove one that exists and one that doesn't
	err = rmCmd.RunE(rmCmd, []string{"rules_go", "nonexistent"})
	require.NoError(t, err)

	// Should warn about nonexistent
	assert.Contains(t, buf.String(), "Warning")
	assert.Contains(t, buf.String(), "nonexistent")
	// Should confirm removal
	assert.Contains(t, buf.String(), "Removed rules_go")

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.NotContains(t, string(content), "rules_go")
	assert.Contains(t, string(content), "rules_python")
}

func TestRmCmd_EmptyDepsAfterRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	rmDryRun = false

	err = rmCmd.RunE(rmCmd, []string{"rules_go"})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	// Should still have module declaration
	assert.Contains(t, string(content), "test_module")
	assert.NotContains(t, string(content), "rules_go")
}

func TestRmCmd_PreservesOtherContent(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `# This is a comment
module(name = "test_module", version = "1.0.0")

# Dependencies
bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")

# Register toolchains
register_toolchains("@rules_go//go:toolchain")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	rmDryRun = false

	err = rmCmd.RunE(rmCmd, []string{"rules_go"})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	// Comments and other content should be preserved
	assert.Contains(t, string(content), "# This is a comment")
	assert.Contains(t, string(content), "# Dependencies")
	assert.Contains(t, string(content), "register_toolchains")
	// The bazel_dep line for rules_go should be gone
	assert.NotContains(t, string(content), `bazel_dep(name = "rules_go"`)
	// But other references (like in register_toolchains) are preserved
	assert.Contains(t, string(content), "@rules_go//go:toolchain")
	assert.Contains(t, string(content), "rules_python")
}
