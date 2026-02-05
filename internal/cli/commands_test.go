package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandDisabledError_Error(t *testing.T) {
	err := &CommandDisabledError{
		Command: "audit",
		Reason:  "disabled in configuration",
	}

	assert.Contains(t, err.Error(), "audit")
	assert.Contains(t, err.Error(), "disabled")
}

func TestCommandDisabledError_Is(t *testing.T) {
	err1 := &CommandDisabledError{Command: "audit", Reason: "test"}
	err2 := &CommandDisabledError{Command: "sync", Reason: "other"}

	// Same type should match
	assert.True(t, err1.Is(err2))
}

func TestCheckCommandAllowed_NotDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	// No config file means no disabled commands
	err := CheckCommandAllowed("info")
	assert.NoError(t, err)
}

func TestCheckCommandAllowed_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	configContent := `
[commands]
disabled = ["audit", "sync"]
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".bzconfig.toml"), []byte(configContent), 0o644))

	err := CheckCommandAllowed("audit")
	require.Error(t, err)

	var cmdErr *CommandDisabledError
	assert.ErrorAs(t, err, &cmdErr)
	assert.Equal(t, "audit", cmdErr.Command)
}

func TestCheckCommandAllowed_OtherCommandNotDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	configContent := `
[commands]
disabled = ["audit", "sync"]
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".bzconfig.toml"), []byte(configContent), 0o644))

	// "info" is not in the disabled list
	err := CheckCommandAllowed("info")
	assert.NoError(t, err)
}

func TestCheckCommandAllowed_DisabledViaEnv(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	// Set environment variable
	t.Setenv("BZ_DISABLE_COMMANDS", "audit,search")

	err := CheckCommandAllowed("audit")
	require.Error(t, err)

	var cmdErr *CommandDisabledError
	assert.ErrorAs(t, err, &cmdErr)
	assert.Equal(t, "audit", cmdErr.Command)
}

func TestCheckOfflineAllowed_NotOffline(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	// Clear global state
	Global = Options{}

	err := CheckOfflineAllowed("audit")
	assert.NoError(t, err)
}

func TestCheckOfflineAllowed_OfflineMode(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	// Set offline mode via global options
	Global = Options{Offline: true}
	t.Cleanup(func() { Global = Options{} })

	err := CheckOfflineAllowed("audit")
	require.Error(t, err)

	var offlineErr *OfflineModeError
	assert.ErrorAs(t, err, &offlineErr)
	assert.Equal(t, "audit", offlineErr.Command)
}

func TestCheckOfflineAllowed_OfflineViaEnv(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	// Set offline via env var
	t.Setenv("BZ_OFFLINE", "1")
	Global = Options{}

	err := CheckOfflineAllowed("audit")
	require.Error(t, err)

	var offlineErr *OfflineModeError
	assert.ErrorAs(t, err, &offlineErr)
	assert.Equal(t, "audit", offlineErr.Command)
}

func TestOfflineModeError_Error(t *testing.T) {
	err := &OfflineModeError{
		Command:     "audit",
		Description: "requires network access to query OSV database",
	}

	errMsg := err.Error()
	assert.Contains(t, errMsg, "audit")
	assert.Contains(t, errMsg, "requires network access")
	assert.Contains(t, errMsg, "offline mode")
}

func TestOfflineModeError_Is(t *testing.T) {
	err1 := &OfflineModeError{Command: "audit"}
	err2 := &OfflineModeError{Command: "sync"}

	// Same type should match
	assert.True(t, err1.Is(err2))
}

func TestCheckOfflineAllowed_OfflineViaConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	configContent := `
[network]
mode = "offline"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".bzconfig.toml"), []byte(configContent), 0o644))

	// Clear global state
	Global = Options{}

	err := CheckOfflineAllowed("audit")
	require.Error(t, err)

	var offlineErr *OfflineModeError
	assert.ErrorAs(t, err, &offlineErr)
}
