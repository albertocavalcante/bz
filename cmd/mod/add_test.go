package mod

import (
	"bytes"
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

	t.Chdir(tmpDir)

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

	t.Chdir(tmpDir)

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

	t.Chdir(tmpDir)

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

	t.Chdir(tmpDir)

	addDev = false

	// Should error without version
	err = addCmd.RunE(addCmd, []string{"rules_go"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestAddCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

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

	t.Chdir(tmpDir)

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

func TestAddCmd_ValidatesModuleExistsInRegistry(t *testing.T) {
	// Create a local test registry with specific modules
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")

	// Create rules_go module with specific versions
	rulesGoDir := filepath.Join(modulesDir, "rules_go")
	require.NoError(t, os.MkdirAll(rulesGoDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(rulesGoDir, "metadata.json"),
		[]byte(`{"versions": ["0.50.0", "0.50.1", "0.51.0"]}`),
		0o644,
	))
	// Create version directory for 0.50.1
	require.NoError(t, os.MkdirAll(filepath.Join(rulesGoDir, "0.50.1"), 0o755))

	// Create gazelle module
	gazelleDir := filepath.Join(modulesDir, "gazelle")
	require.NoError(t, os.MkdirAll(gazelleDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(gazelleDir, "metadata.json"),
		[]byte(`{"versions": ["0.38.0"]}`),
		0o644,
	))

	// Create MODULE.bazel in temp dir
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	// Save and restore registry flag and other flags
	oldRegistry := registryFlag
	registryFlag = root
	addDev = false
	addNoVerify = false
	defer func() {
		registryFlag = oldRegistry
		addDev = false
		addNoVerify = false
	}()

	// Test: Adding a valid module should succeed
	t.Run("valid module succeeds", func(t *testing.T) {
		// Reset file
		require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

		err := addCmd.RunE(addCmd, []string{"rules_go@0.50.1"})
		require.NoError(t, err)

		content, err := os.ReadFile(modulePath)
		require.NoError(t, err)
		assert.Contains(t, string(content), `bazel_dep(name = "rules_go", version = "0.50.1")`)
	})

	// Test: Adding non-existent module should fail with helpful error
	t.Run("non-existent module fails with suggestion", func(t *testing.T) {
		// Reset file
		require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

		err := addCmd.RunE(addCmd, []string{"rule_go@0.50.1"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `module "rule_go" not found`)
		assert.Contains(t, err.Error(), "Did you mean")
		assert.Contains(t, err.Error(), "rules_go")
	})

	// Test: Adding module with non-existent version should fail
	t.Run("non-existent version fails with available versions", func(t *testing.T) {
		// Reset file
		require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

		err := addCmd.RunE(addCmd, []string{"rules_go@999.0.0"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `version "999.0.0" not found`)
		assert.Contains(t, err.Error(), "rules_go")
		assert.Contains(t, err.Error(), "Available versions")
		// Should show actual versions
		assert.Contains(t, err.Error(), "0.51.0")
	})
}

func TestAddCmd_NoVerifySkipsValidation(t *testing.T) {
	// Create a local test registry with limited modules
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")

	// Create only gazelle module (not rules_go)
	gazelleDir := filepath.Join(modulesDir, "gazelle")
	require.NoError(t, os.MkdirAll(gazelleDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(gazelleDir, "metadata.json"),
		[]byte(`{"versions": ["0.38.0"]}`),
		0o644,
	))

	// Create MODULE.bazel in temp dir
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	// Save and restore registry flag and other flags
	oldRegistry := registryFlag
	registryFlag = root
	addDev = false
	addNoVerify = true // Enable --no-verify
	defer func() {
		registryFlag = oldRegistry
		addDev = false
		addNoVerify = false
	}()

	// With --no-verify, adding a non-existent module should succeed
	err := addCmd.RunE(addCmd, []string{"nonexistent_module@1.0.0"})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `bazel_dep(name = "nonexistent_module", version = "1.0.0")`)
}

func TestAddCmd_ValidationShowsSearchHint(t *testing.T) {
	// Create a local test registry with no similar modules
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")

	// Create a module with completely different name
	differentDir := filepath.Join(modulesDir, "totally_different")
	require.NoError(t, os.MkdirAll(differentDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(differentDir, "metadata.json"),
		[]byte(`{"versions": ["1.0.0"]}`),
		0o644,
	))

	// Create MODULE.bazel in temp dir
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(modulePath, []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	// Save and restore registry flag and other flags
	oldRegistry := registryFlag
	registryFlag = root
	addDev = false
	addNoVerify = false
	defer func() {
		registryFlag = oldRegistry
		addDev = false
		addNoVerify = false
	}()

	// Non-existent module with no close matches should show search hint
	err := addCmd.RunE(addCmd, []string{"nonexistent@1.0.0"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `module "nonexistent" not found`)
	assert.Contains(t, err.Error(), "bz mod search")
}

func TestAddCmd_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	t.Chdir(tmpDir)

	// Reset flags
	addDev = false
	addDryRun = true
	addNoVerify = true // Skip verification for this test
	defer func() {
		addDryRun = false
		addNoVerify = false
	}()

	var stdout bytes.Buffer
	addCmd.SetOut(&stdout)

	err = addCmd.RunE(addCmd, []string{"rules_go@0.50.1"})
	require.NoError(t, err)

	// Verify output indicates dry-run
	output := stdout.String()
	assert.Contains(t, output, "Would add")
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.50.1")

	// Verify the file was NOT modified
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "rules_go", "File should not be modified in dry-run mode")
}

func TestAddCmd_DryRunMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte(moduleContent), 0o644)
	require.NoError(t, err)

	t.Chdir(tmpDir)

	addDev = false
	addDryRun = true
	addNoVerify = true // Skip verification for this test
	defer func() {
		addDryRun = false
		addNoVerify = false
	}()

	var stdout bytes.Buffer
	addCmd.SetOut(&stdout)

	err = addCmd.RunE(addCmd, []string{"rules_go@0.50.1", "rules_python@0.35.0"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "rules_python")

	// Verify file unchanged
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "rules_go")
	assert.NotContains(t, string(content), "rules_python")
}
