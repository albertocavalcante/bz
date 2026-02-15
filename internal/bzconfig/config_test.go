package bzconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromTOML(t *testing.T) {
	t.Parallel()
	t.Run("loads complete config from TOML file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".bzconfig.toml")

		configContent := `
[network]
mode = "prefer-offline"
registry = "https://internal-registry.example.com"
fallback_registries = ["https://bcr.bazel.build"]
timeout = "60s"

[cache]
dir = "/custom/cache/path"
ttl = "24h"

[commands]
disabled = ["audit", "search"]
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		cfg, err := LoadFromFile(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.Equal(t, NetworkModePreferOffline, cfg.Network.Mode)
		assert.Equal(t, "https://internal-registry.example.com", cfg.Network.Registry)
		assert.Equal(t, []string{"https://bcr.bazel.build"}, cfg.Network.FallbackRegistries)
		assert.Equal(t, "60s", cfg.Network.Timeout)
		assert.Equal(t, "/custom/cache/path", cfg.Cache.Dir)
		assert.Equal(t, "24h", cfg.Cache.TTL)
		assert.Equal(t, []string{"audit", "search"}, cfg.Commands.Disabled)
	})

	t.Run("loads partial config with defaults", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")

		configContent := `
[network]
mode = "offline"
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		cfg, err := LoadFromFile(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.Equal(t, NetworkModeOffline, cfg.Network.Mode)
		// Check defaults are applied
		assert.Equal(t, "https://bcr.bazel.build", cfg.Network.Registry)
		assert.Equal(t, "30s", cfg.Network.Timeout)
	})

	t.Run("returns error for invalid TOML", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")

		configContent := `this is not valid toml [[[`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		_, err = LoadFromFile(configPath)
		require.Error(t, err)
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		t.Parallel()
		_, err := LoadFromFile("/nonexistent/path/config.toml")
		require.Error(t, err)
	})
}

func TestLoadWithPrecedence(t *testing.T) {
	t.Run("CLI overrides take highest precedence", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".bzconfig.toml")

		configContent := `
[network]
mode = "online"
registry = "https://file-registry.example.com"
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		overrides := &Config{
			Network: NetworkConfig{
				Mode:     NetworkModeOffline,
				Registry: "https://override-registry.example.com",
			},
		}

		cfg, err := Load(overrides, WithProjectConfig(configPath))
		require.NoError(t, err)

		// Overrides should win
		assert.Equal(t, NetworkModeOffline, cfg.Network.Mode)
		assert.Equal(t, "https://override-registry.example.com", cfg.Network.Registry)
	})

	t.Run("environment variables override file config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".bzconfig.toml")

		configContent := `
[network]
mode = "online"
registry = "https://file-registry.example.com"
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		// Set environment variables
		t.Setenv("BZ_OFFLINE", "1")
		t.Setenv("BZ_REGISTRY", "https://env-registry.example.com")

		cfg, err := Load(nil, WithProjectConfig(configPath))
		require.NoError(t, err)

		// Env vars should override file
		assert.Equal(t, NetworkModeOffline, cfg.Network.Mode)
		assert.Equal(t, "https://env-registry.example.com", cfg.Network.Registry)
	})

	t.Run("project config overrides user config", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		userConfig := filepath.Join(tmpDir, "user-config.toml")
		projectConfig := filepath.Join(tmpDir, "project-config.toml")

		userContent := `
[network]
mode = "online"
registry = "https://user-registry.example.com"
timeout = "10s"
`
		projectContent := `
[network]
mode = "prefer-offline"
registry = "https://project-registry.example.com"
`
		err := os.WriteFile(userConfig, []byte(userContent), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(projectConfig, []byte(projectContent), 0o644)
		require.NoError(t, err)

		cfg, err := Load(nil,
			WithUserConfig(userConfig),
			WithProjectConfig(projectConfig),
		)
		require.NoError(t, err)

		// Project config should override user config
		assert.Equal(t, NetworkModePreferOffline, cfg.Network.Mode)
		assert.Equal(t, "https://project-registry.example.com", cfg.Network.Registry)
		// But user config values not overridden should remain
		assert.Equal(t, "10s", cfg.Network.Timeout)
	})

	t.Run("defaults when no config exists", func(t *testing.T) {
		t.Parallel()
		cfg, err := Load(nil)
		require.NoError(t, err)

		assert.Equal(t, NetworkModeOnline, cfg.Network.Mode)
		assert.Equal(t, "https://bcr.bazel.build", cfg.Network.Registry)
		assert.Equal(t, "30s", cfg.Network.Timeout)
		assert.Empty(t, cfg.Commands.Disabled)
	})
}

func TestEnvironmentVariables(t *testing.T) {
	t.Run("BZ_OFFLINE sets offline mode", func(t *testing.T) {
		t.Setenv("BZ_OFFLINE", "1")

		cfg, err := Load(nil)
		require.NoError(t, err)
		assert.Equal(t, NetworkModeOffline, cfg.Network.Mode)
	})

	t.Run("BZ_PREFER_OFFLINE sets prefer-offline mode", func(t *testing.T) {
		t.Setenv("BZ_PREFER_OFFLINE", "1")

		cfg, err := Load(nil)
		require.NoError(t, err)
		assert.Equal(t, NetworkModePreferOffline, cfg.Network.Mode)
	})

	t.Run("BZ_OFFLINE takes precedence over BZ_PREFER_OFFLINE", func(t *testing.T) {
		t.Setenv("BZ_OFFLINE", "1")
		t.Setenv("BZ_PREFER_OFFLINE", "1")

		cfg, err := Load(nil)
		require.NoError(t, err)
		assert.Equal(t, NetworkModeOffline, cfg.Network.Mode)
	})

	t.Run("BZ_REGISTRY overrides registry", func(t *testing.T) {
		t.Setenv("BZ_REGISTRY", "https://custom-registry.example.com")

		cfg, err := Load(nil)
		require.NoError(t, err)
		assert.Equal(t, "https://custom-registry.example.com", cfg.Network.Registry)
	})

	t.Run("BZ_CACHE_DIR overrides cache directory", func(t *testing.T) {
		t.Setenv("BZ_CACHE_DIR", "/custom/cache/dir")

		cfg, err := Load(nil)
		require.NoError(t, err)
		assert.Equal(t, "/custom/cache/dir", cfg.Cache.Dir)
	})

	t.Run("BZ_DISABLE_COMMANDS adds disabled commands", func(t *testing.T) {
		t.Setenv("BZ_DISABLE_COMMANDS", "audit,search,sync")

		cfg, err := Load(nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"audit", "search", "sync"}, cfg.Commands.Disabled)
	})

	t.Run("BZ_DISABLE_COMMANDS handles whitespace", func(t *testing.T) {
		t.Setenv("BZ_DISABLE_COMMANDS", " audit , search , sync ")

		cfg, err := Load(nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"audit", "search", "sync"}, cfg.Commands.Disabled)
	})
}

func TestIsOffline(t *testing.T) {
	t.Parallel()
	t.Run("returns true for offline mode", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Network: NetworkConfig{Mode: NetworkModeOffline}}
		assert.True(t, cfg.IsOffline())
	})

	t.Run("returns false for online mode", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Network: NetworkConfig{Mode: NetworkModeOnline}}
		assert.False(t, cfg.IsOffline())
	})

	t.Run("returns false for prefer-offline mode", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Network: NetworkConfig{Mode: NetworkModePreferOffline}}
		assert.False(t, cfg.IsOffline())
	})
}

func TestIsPreferOffline(t *testing.T) {
	t.Parallel()
	t.Run("returns true for prefer-offline mode", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Network: NetworkConfig{Mode: NetworkModePreferOffline}}
		assert.True(t, cfg.IsPreferOffline())
	})

	t.Run("returns false for online mode", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Network: NetworkConfig{Mode: NetworkModeOnline}}
		assert.False(t, cfg.IsPreferOffline())
	})

	t.Run("returns false for offline mode", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Network: NetworkConfig{Mode: NetworkModeOffline}}
		assert.False(t, cfg.IsPreferOffline())
	})
}

func TestIsCommandDisabled(t *testing.T) {
	t.Parallel()
	t.Run("returns true for disabled command", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Commands: CommandsConfig{Disabled: []string{"audit", "search"}}}
		assert.True(t, cfg.IsCommandDisabled("audit"))
		assert.True(t, cfg.IsCommandDisabled("search"))
	})

	t.Run("returns false for enabled command", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Commands: CommandsConfig{Disabled: []string{"audit"}}}
		assert.False(t, cfg.IsCommandDisabled("sync"))
		assert.False(t, cfg.IsCommandDisabled("version"))
	})

	t.Run("returns false when no commands disabled", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Commands: CommandsConfig{}}
		assert.False(t, cfg.IsCommandDisabled("audit"))
	})
}

func TestGetRegistry(t *testing.T) {
	t.Parallel()
	t.Run("returns configured registry", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Network: NetworkConfig{Registry: "https://custom.example.com"}}
		assert.Equal(t, "https://custom.example.com", cfg.GetRegistry())
	})

	t.Run("returns default registry when empty", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{}
		cfg.applyDefaults()
		assert.Equal(t, "https://bcr.bazel.build", cfg.GetRegistry())
	})
}

func TestNetworkModeConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, NetworkMode("online"), NetworkModeOnline)
	assert.Equal(t, NetworkMode("prefer-offline"), NetworkModePreferOffline)
	assert.Equal(t, NetworkMode("offline"), NetworkModeOffline)
}

func TestLoadWithSystemConfig(t *testing.T) {
	t.Parallel()
	t.Run("system config is lowest priority file config", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		systemConfig := filepath.Join(tmpDir, "system-config.toml")
		userConfig := filepath.Join(tmpDir, "user-config.toml")

		systemContent := `
[network]
mode = "offline"
registry = "https://system-registry.example.com"
timeout = "120s"

[cache]
ttl = "48h"
`
		userContent := `
[network]
mode = "online"
`
		err := os.WriteFile(systemConfig, []byte(systemContent), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(userConfig, []byte(userContent), 0o644)
		require.NoError(t, err)

		cfg, err := Load(nil,
			WithSystemConfig(systemConfig),
			WithUserConfig(userConfig),
		)
		require.NoError(t, err)

		// User config should override system config mode
		assert.Equal(t, NetworkModeOnline, cfg.Network.Mode)
		// System config values not overridden should remain
		assert.Equal(t, "https://system-registry.example.com", cfg.Network.Registry)
		assert.Equal(t, "120s", cfg.Network.Timeout)
		assert.Equal(t, "48h", cfg.Cache.TTL)
	})
}

func TestDefaultCacheDir(t *testing.T) {
	t.Parallel()
	t.Run("default cache dir uses home directory", func(t *testing.T) {
		t.Parallel()
		cfg, err := Load(nil)
		require.NoError(t, err)

		home, err := os.UserHomeDir()
		require.NoError(t, err)
		expected := filepath.Join(home, ".cache", "bz")
		assert.Equal(t, expected, cfg.Cache.Dir)
	})
}
