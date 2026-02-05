package mod

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/registry"
)

func TestInfoCmd_RequiresModuleArg(t *testing.T) {
	err := runInfo(infoCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "module argument required")
}

func TestInfoCmd_ShowsMetadataWithoutVersion(t *testing.T) {
	// This is an integration test - skip in CI if needed
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	infoJSON = false

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	// Without version, should show metadata
	err := runInfo(infoCmd, []string{"rules_go"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "Latest:")
	assert.Contains(t, output, "Versions:")
}

func TestInfoCmd_FetchesFromRegistry(t *testing.T) {
	// This is an integration test - skip in CI if needed
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	infoJSON = false

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	// Use a real module that exists in BCR
	err := runInfo(infoCmd, []string{"rules_go@0.50.1"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.50.1")
}

func TestInfoCmd_JSONOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	infoJSON = true
	defer func() { infoJSON = false }()

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	err := runInfo(infoCmd, []string{"rules_go@0.50.1"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, `"name"`)
	assert.Contains(t, output, `"dependencies"`)
}

func TestInfoCmd_SuggestsModuleOnTypo(t *testing.T) {
	// Create a local test registry
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")

	// Create some test modules
	modules := []string{"rules_go", "rules_python", "gazelle"}
	for _, m := range modules {
		dir := filepath.Join(modulesDir, m)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		metaPath := filepath.Join(dir, "metadata.json")
		require.NoError(t, os.WriteFile(metaPath, []byte(`{"versions": ["1.0.0"]}`), 0o644))
	}

	// Save and restore the registry flag
	oldRegistry := registryFlag
	registryFlag = root
	defer func() { registryFlag = oldRegistry }()

	infoJSON = false

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	// Use a typo that's close to "rules_go"
	err := runInfo(infoCmd, []string{"rule_go"})

	// Should return an error
	require.Error(t, err)

	// Error should be ErrModuleNotFound
	assert.True(t, errors.Is(err, registry.ErrModuleNotFound))

	// Error message should contain the module name and suggestion
	assert.Contains(t, err.Error(), `module "rule_go" not found`)
	assert.Contains(t, err.Error(), "Did you mean")
	assert.Contains(t, err.Error(), "rules_go")
}

func TestInfoCmd_ModuleNotFoundWithoutSuggestions(t *testing.T) {
	// Create a local test registry with no similar modules
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules")

	// Create a module that's very different from the query
	dir := filepath.Join(modulesDir, "totally_different_name")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	metaPath := filepath.Join(dir, "metadata.json")
	require.NoError(t, os.WriteFile(metaPath, []byte(`{"versions": ["1.0.0"]}`), 0o644))

	// Save and restore the registry flag
	oldRegistry := registryFlag
	registryFlag = root
	defer func() { registryFlag = oldRegistry }()

	infoJSON = false

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	// Query for something very different
	err := runInfo(infoCmd, []string{"rules_go"})

	// Should return an error
	require.Error(t, err)

	// Error should be ErrModuleNotFound
	assert.True(t, errors.Is(err, registry.ErrModuleNotFound))

	// Error message should contain the module name but NOT suggestions
	assert.Contains(t, err.Error(), `module "rules_go" not found`)
	assert.NotContains(t, err.Error(), "Did you mean")
}

func TestInfoCmd_DisabledViaConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	// Create config that disables info command
	configContent := `
[commands]
disabled = ["info"]
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".bzconfig.toml"), []byte(configContent), 0o644))

	infoJSON = false

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	err := runInfo(infoCmd, []string{"rules_go"})
	require.Error(t, err)

	// Error message should indicate command is disabled
	errMsg := err.Error()
	assert.Contains(t, errMsg, "info")
	assert.Contains(t, errMsg, "disabled")
}

func TestInfoCmd_OfflineMode(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	// Set offline mode via environment variable
	t.Setenv("BZ_OFFLINE", "1")

	infoJSON = false

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	err := runInfo(infoCmd, []string{"rules_go"})
	require.Error(t, err)

	// Error should be a network error (cache miss in offline mode)
	// The error contains "offline mode" in its Operation field
	var netErr *registry.NetworkError
	if errors.As(err, &netErr) {
		assert.Contains(t, netErr.Operation, "offline mode")
	} else {
		// If not a network error, check wrapping
		assert.True(t, errors.Is(err, registry.ErrModuleNotFound), "expected network error or module not found")
	}
}
