package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/bzconfig"
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

func TestCheckCommandEnabled(t *testing.T) {
	commands := []string{"audit", "cache", "cache stats", "mod", "mod info", "mod stats", "registry", "registry ping", "version"}
	disabled := []string{"registry ping", "info"}
	eff, err := bzconfig.Merge(bzconfig.Defaults("/c"), bzconfig.Layer{
		Kind:     bzconfig.LayerProject,
		Origin:   "/ws/.bz.toml",
		Settings: bzconfig.Settings{Disabled: &disabled},
	})
	require.NoError(t, err)

	_, err = CheckCommandEnabled(eff, "registry ping", commands)
	var cmdErr *CommandDisabledError
	require.ErrorAs(t, err, &cmdErr)
	assert.Equal(t, "registry ping", cmdErr.Command)
	assert.Contains(t, err.Error(), "/ws/.bz.toml")

	_, err = CheckCommandEnabled(eff, "mod info", commands)
	require.ErrorAs(t, err, &cmdErr)
	assert.Contains(t, err.Error(), `listed as "info"`)

	_, err = CheckCommandEnabled(eff, "audit", commands)
	assert.NoError(t, err)

	_, err = CheckCommandEnabled(eff, "version", commands)
	assert.NoError(t, err)
}

func TestCheckCommandEnabled_DisabledViaEnv(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("BZ_DISABLE_COMMANDS", "audit,search")
	eff, _, err := LoadEffective()
	require.NoError(t, err)
	t.Cleanup(ResetCachedConfig)

	_, err = CheckCommandEnabled(eff, "audit", []string{"audit", "mod search"})
	var cmdErr *CommandDisabledError
	require.ErrorAs(t, err, &cmdErr)
	assert.Contains(t, err.Error(), "environment BZ_DISABLE_COMMANDS")
}

func TestCheckOfflineAllowed_NotOffline(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))
	ResetCachedConfig()
	t.Cleanup(ResetCachedConfig)

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
	ResetCachedConfig()
	t.Cleanup(ResetCachedConfig)

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
	ResetCachedConfig()
	t.Cleanup(ResetCachedConfig)

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
	ResetCachedConfig()
	t.Cleanup(ResetCachedConfig)

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
	assert.Contains(t, err.Error(), "project config")
	assert.Contains(t, err.Error(), ".bzconfig.toml")
	assert.NotContains(t, err.Error(), "bz.star")
}

func TestEffective_FlagBeatsConfigMode(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	ResetCachedConfig()
	t.Cleanup(ResetCachedConfig)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".bz.toml"), []byte("[network]\nmode = \"offline\"\n"), 0o600))
	Global = Options{PreferOffline: true}
	t.Cleanup(func() { Global = Options{} })

	eff, err := Effective()
	require.NoError(t, err)
	assert.Equal(t, bzconfig.NetworkModePreferOffline, eff.Network.Mode)
	assert.Equal(t, "flag --prefer-offline", eff.Source(bzconfig.KeyMode).String())
}

func TestCheckOfflineAllowed_InvalidConfigFailsClosed(t *testing.T) {
	// The error names the resolved path (/private/var on macOS).
	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Chdir(tmpDir)
	ResetCachedConfig()
	t.Cleanup(ResetCachedConfig)
	Global = Options{}
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".bz.toml"), []byte("[network]\nmode = \"ofline\"\n"), 0o600))

	err = CheckOfflineAllowed("audit")
	require.EqualError(t, err, `invalid network mode "ofline" in project config `+filepath.Join(tmpDir, ".bz.toml")+` (want "online", "prefer-offline" or "offline")`)
}

// TestLoadEffective_Precedence runs every layer through the path the binary
// takes: the cached environment, LoadLayers and the flag layer from Global.
func TestLoadEffective_Precedence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ownership rules differ on Windows")
	}
	configHome, ws := t.TempDir(), t.TempDir()
	userPath := filepath.Join(configHome, "bz", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(userPath), 0o700))
	require.NoError(t, os.WriteFile(userPath, []byte("[network]\nmode = \"offline\"\nregistry = \"https://user.example\"\n[cache]\nttl = \"1h\"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(ws, "MODULE.bazel"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(ws, ".bz.toml"), []byte("[network]\nregistry = \"https://project.example\"\ntimeout = \"30s\"\n"), 0o600))
	t.Chdir(ws)
	t.Cleanup(SetEnvironment(bzconfig.OSEnvironment().Isolated(configHome).WithEnv(map[string]string{
		bzconfig.EnvCacheDir: "/from/env", bzconfig.EnvOffline: "", bzconfig.EnvPreferOffline: "", bzconfig.EnvRegistry: "",
		bzconfig.EnvDisableCommands: "", bzconfig.EnvTrustedConfigDirs: "",
	})))
	Global = Options{PreferOffline: true}
	t.Cleanup(func() { Global = Options{} })

	eff, warnings, err := LoadEffective()
	require.NoError(t, err)
	assert.Empty(t, warnings)
	for key, want := range map[bzconfig.Key]bzconfig.LayerKind{
		bzconfig.KeyMode:     bzconfig.LayerFlag,
		bzconfig.KeyRegistry: bzconfig.LayerProject,
		bzconfig.KeyTimeout:  bzconfig.LayerProject,
		bzconfig.KeyCacheDir: bzconfig.LayerEnv,
		bzconfig.KeyCacheTTL: bzconfig.LayerUser,
	} {
		assert.Equal(t, want, eff.Source(key).Layer, key)
	}
	assert.Equal(t, bzconfig.NetworkModePreferOffline, eff.Network.Mode)
	assert.Equal(t, "https://project.example", eff.Network.Registry)
	assert.Equal(t, "/from/env", eff.Cache.Dir)
	assert.Equal(t, "1h", eff.Cache.TTL)
}

// TestLoadEffective_InvalidConfigFailsClosed: the load error the root
// command enforces with (see cmd TestRun_ConfigLoadErrorFailsClosed).
func TestLoadEffective_InvalidConfigFailsClosed(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile(".bzconfig.toml", []byte("[network]\ntimeout = \"30\"\n[commands]\ndisabled = [\"audit\"]\n"), 0o644))
	t.Cleanup(ResetCachedConfig)

	eff, _, err := LoadEffective()
	require.Error(t, err)
	assert.Nil(t, eff)
	assert.Contains(t, err.Error(), `invalid network timeout "30"`)
}
