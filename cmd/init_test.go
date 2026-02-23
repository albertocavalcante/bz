package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/module"
)

func TestInitCmd_CreatesModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	// Reset flags
	initName = "test_module"
	initVersion = "0.0.0"
	initForce = false
	defer func() {
		initName = ""
		initVersion = "0.0.0"
		initForce = false
	}()

	var buf bytes.Buffer
	initCmd.SetOut(&buf)
	defer initCmd.SetOut(os.Stdout)

	err := initCmd.RunE(initCmd, []string{})
	require.NoError(t, err)

	// Verify file was created
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `name = "test_module"`)
	assert.Contains(t, string(content), `version = "0.0.0"`)
	assert.Contains(t, buf.String(), "Created MODULE.bazel")
}

func TestInitCmd_UsesCustomVersion(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	initName = "my_module"
	initVersion = "1.0.0"
	initForce = false
	defer func() {
		initName = ""
		initVersion = "0.0.0"
		initForce = false
	}()

	err := initCmd.RunE(initCmd, []string{})
	require.NoError(t, err)

	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `version = "1.0.0"`)
}

func TestInitCmd_UsesDirectoryNameWhenNoName(t *testing.T) {
	// Create a temp dir with a known name
	parentDir := t.TempDir()
	testDir := filepath.Join(parentDir, "my_project")
	require.NoError(t, os.Mkdir(testDir, 0o755))

	t.Chdir(testDir)

	initName = "" // No name provided
	initVersion = "0.0.0"
	initForce = false
	defer func() {
		initName = ""
		initVersion = "0.0.0"
		initForce = false
	}()

	err := initCmd.RunE(initCmd, []string{})
	require.NoError(t, err)

	modulePath := filepath.Join(testDir, "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `name = "my_project"`)
}

func TestInitCmd_ErrorsOnExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte("existing content"), 0o644)
	require.NoError(t, err)

	t.Chdir(tmpDir)

	initName = "test_module"
	initVersion = "0.0.0"
	initForce = false
	defer func() {
		initName = ""
		initVersion = "0.0.0"
		initForce = false
	}()

	err = initCmd.RunE(initCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Contains(t, err.Error(), "--force")
}

func TestInitCmd_ForceOverwritesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	err := os.WriteFile(modulePath, []byte("existing content"), 0o644)
	require.NoError(t, err)

	t.Chdir(tmpDir)

	initName = "new_module"
	initVersion = "2.0.0"
	initForce = true
	defer func() {
		initName = ""
		initVersion = "0.0.0"
		initForce = false
	}()

	err = initCmd.RunE(initCmd, []string{})
	require.NoError(t, err)

	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.NotContains(t, string(content), "existing content")
	assert.Contains(t, string(content), `name = "new_module"`)
	assert.Contains(t, string(content), `version = "2.0.0"`)
}

func TestInitCmd_InvalidModuleName(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	tests := []struct {
		name        string
		moduleName  string
		wantErr     bool
		errContains string
	}{
		{
			name:        "starts with number",
			moduleName:  "123module",
			wantErr:     true,
			errContains: "must start with a lowercase letter",
		},
		{
			name:        "uppercase letter",
			moduleName:  "MyModule",
			wantErr:     true,
			errContains: "must start with a lowercase letter",
		},
		{
			name:        "contains space",
			moduleName:  "my module",
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:       "empty name",
			moduleName: "",
			wantErr:    false, // Will use directory name
		},
		{
			name:       "valid with underscore",
			moduleName: "my_module",
			wantErr:    false,
		},
		{
			name:       "valid with dot",
			moduleName: "my.module",
			wantErr:    false,
		},
		{
			name:       "valid with dash",
			moduleName: "my-module",
			wantErr:    false,
		},
		{
			name:       "valid with number",
			moduleName: "module123",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up from previous test
			modulePath := filepath.Join(tmpDir, "MODULE.bazel")
			_ = os.Remove(modulePath)

			initName = tt.moduleName
			initVersion = "0.0.0"
			initForce = false

			err := initCmd.RunE(initCmd, []string{})
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}

	// Reset flags
	initName = ""
	initVersion = "0.0.0"
	initForce = false
}

func TestSanitizeModuleName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my_project", "my_project"},
		{"MyProject", "myproject"},
		{"my-project", "my-project"},
		{"my.project", "my.project"},
		{"123project", "module__123project"},
		{"_project", "module__project"},
		{"my project", "my_project"},
		{"My Project!", "my_project_"},
		{"@special#chars", "module__special_chars"},
		{"", "my_module"},
		{"project123", "project123"},
		{"PROJECT", "project"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := module.SanitizeModuleName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateModuleName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid", false},
		{"my_module", false},
		{"my-module", false},
		{"my.module", false},
		{"module123", false},
		{"a", false},
		{"", true},          // empty
		{"1module", true},   // starts with number
		{"_module", true},   // starts with underscore
		{"Module", true},    // starts with uppercase
		{"my Module", true}, // contains space
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.ValidateModuleName(tt.name)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatModuleFile(t *testing.T) {
	content := module.FormatModuleFile("my_module", "1.0.0")

	expected := `module(
    name = "my_module",
    version = "1.0.0",
)
`
	assert.Equal(t, expected, content)
}

func TestInitCmd_OutputMessage(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	initName = "awesome_module"
	initVersion = "1.2.3"
	initForce = false
	defer func() {
		initName = ""
		initVersion = "0.0.0"
		initForce = false
	}()

	var buf bytes.Buffer
	initCmd.SetOut(&buf)
	defer initCmd.SetOut(os.Stdout)

	err := initCmd.RunE(initCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "awesome_module")
	assert.Contains(t, output, "1.2.3")
}

func TestInitCmd_SanitizesDirectoryName(t *testing.T) {
	// Create a temp dir with a name that needs sanitization
	parentDir := t.TempDir()
	testDir := filepath.Join(parentDir, "My Project 123")
	require.NoError(t, os.Mkdir(testDir, 0o755))

	t.Chdir(testDir)

	initName = "" // No name provided, use directory name
	initVersion = "0.0.0"
	initForce = false
	defer func() {
		initName = ""
		initVersion = "0.0.0"
		initForce = false
	}()

	err := initCmd.RunE(initCmd, []string{})
	require.NoError(t, err)

	modulePath := filepath.Join(testDir, "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	// Should be sanitized: "My Project 123" -> "my_project_123"
	assert.Contains(t, string(content), `name = "my_project_123"`)
}

func TestInitCmd_PreservesValidSpecialChars(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	initName = "my-module.v2_test"
	initVersion = "0.0.0"
	initForce = false
	defer func() {
		initName = ""
		initVersion = "0.0.0"
		initForce = false
	}()

	err := initCmd.RunE(initCmd, []string{})
	require.NoError(t, err)

	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `name = "my-module.v2_test"`)
}

func TestInitCmd_ModuleFileFormat(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	initName = "test"
	initVersion = "1.0.0"
	initForce = false
	defer func() {
		initName = ""
		initVersion = "0.0.0"
		initForce = false
	}()

	err := initCmd.RunE(initCmd, []string{})
	require.NoError(t, err)

	modulePath := filepath.Join(tmpDir, "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	require.NoError(t, err)

	// Verify proper Starlark formatting
	expected := `module(
    name = "test",
    version = "1.0.0",
)
`
	assert.Equal(t, expected, string(content))
}
