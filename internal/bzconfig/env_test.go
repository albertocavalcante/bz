package bzconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEnvTrue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"1 is true", "1", true},
		{"true is true", "true", true},
		{"TRUE is true", "TRUE", true},
		{"True is true", "True", true},
		{"yes is true", "yes", true},
		{"YES is true", "YES", true},
		{"Yes is true", "Yes", true},
		{"0 is false", "0", false},
		{"false is false", "false", false},
		{"no is false", "no", false},
		{"empty is false", "", false},
		{"random string is false", "random", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envName := "TEST_ENV_VAR_" + tt.name
			t.Setenv(envName, tt.value)
			assert.Equal(t, tt.expected, isEnvTrue(envName))
		})
	}
}

func TestParseCommaSeparated(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple list",
			input:    "a,b,c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "list with spaces",
			input:    " a , b , c ",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "single item",
			input:    "single",
			expected: []string{"single"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only commas",
			input:    ",,",
			expected: []string{},
		},
		{
			name:     "commas with spaces",
			input:    " , , ",
			expected: []string{},
		},
		{
			name:     "mixed empty and values",
			input:    "a,,b,  ,c",
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseCommaSeparated(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyEnvVarsIntegration(t *testing.T) {
	t.Run("all environment variables applied", func(t *testing.T) {
		t.Setenv("BZ_OFFLINE", "1")
		t.Setenv("BZ_REGISTRY", "https://env.example.com")
		t.Setenv("BZ_CACHE_DIR", "/env/cache")
		t.Setenv("BZ_DISABLE_COMMANDS", "cmd1,cmd2")

		cfg := &Config{}
		cfg.applyDefaults()
		cfg.applyEnvVars()

		assert.Equal(t, NetworkModeOffline, cfg.Network.Mode)
		assert.Equal(t, "https://env.example.com", cfg.Network.Registry)
		assert.Equal(t, "/env/cache", cfg.Cache.Dir)
		assert.Equal(t, []string{"cmd1", "cmd2"}, cfg.Commands.Disabled)
	})

	t.Run("prefer-offline when offline not set", func(t *testing.T) {
		t.Setenv("BZ_PREFER_OFFLINE", "true")

		cfg := &Config{}
		cfg.applyDefaults()
		cfg.applyEnvVars()

		assert.Equal(t, NetworkModePreferOffline, cfg.Network.Mode)
	})

	t.Run("no changes when env vars not set", func(t *testing.T) {
		cfg := &Config{}
		cfg.applyDefaults()
		originalMode := cfg.Network.Mode
		originalRegistry := cfg.Network.Registry
		originalCacheDir := cfg.Cache.Dir

		cfg.applyEnvVars()

		assert.Equal(t, originalMode, cfg.Network.Mode)
		assert.Equal(t, originalRegistry, cfg.Network.Registry)
		assert.Equal(t, originalCacheDir, cfg.Cache.Dir)
	})
}
