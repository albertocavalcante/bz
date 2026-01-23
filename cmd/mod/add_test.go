package mod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddCmd_AddsNewDependency(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Reset flags
	addDev = false

	err = addCmd.RunE(addCmd, []string{"rules_go@0.50.1"})
	require.NoError(t, err)

	// Verify the file was modified
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `bazel_dep(name = "rules_go", version = "0.50.1")`)
}

func TestAddCmd_AddsDevDependency(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Set dev flag
	addDev = true
	defer func() { addDev = false }()

	err = addCmd.RunE(addCmd, []string{"gazelle@0.38.0"})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)`)
}

func TestAddCmd_SkipsExistingDependency(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	addDev = false

	// Should not error, just skip
	err = addCmd.RunE(addCmd, []string{"rules_go@0.50.1"})
	require.NoError(t, err)

	// File should NOT have duplicate
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	// Should still have original version, not added new one
	assert.Contains(t, string(content), `version = "0.50.0"`)
}

func TestAddCmd_RequiresVersion(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	err := os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	addDev = false

	// Should error without version
	err = addCmd.RunE(addCmd, []string{"rules_go"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestAddCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	addDev = false

	err := addCmd.RunE(addCmd, []string{"rules_go@0.50.1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestAddCmd_MultipleModules(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	addDev = false

	err = addCmd.RunE(addCmd, []string{"rules_go@0.50.1", "rules_python@0.35.0"})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `bazel_dep(name = "rules_go", version = "0.50.1")`)
	assert.Contains(t, string(content), `bazel_dep(name = "rules_python", version = "0.35.0")`)
}

func TestParseModuleArg(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{"rules_go@0.50.1", "rules_go", "0.50.1"},
		{"rules_go", "rules_go", ""},
		{"@scope/pkg@1.0.0", "@scope/pkg", "1.0.0"},
		{"pkg@1.0.0-rc1", "pkg", "1.0.0-rc1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, version := parseModuleArg(tt.input)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantVersion, version)
		})
	}
}
