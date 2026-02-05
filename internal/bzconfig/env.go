package bzconfig

import (
	"os"
	"strings"
)

// Environment variable names.
const (
	EnvOffline         = "BZ_OFFLINE"
	EnvPreferOffline   = "BZ_PREFER_OFFLINE"
	EnvRegistry        = "BZ_REGISTRY"
	EnvCacheDir        = "BZ_CACHE_DIR"
	EnvDisableCommands = "BZ_DISABLE_COMMANDS"
)

// applyEnvVars applies environment variable overrides to the config.
func (c *Config) applyEnvVars() {
	// BZ_OFFLINE takes precedence over BZ_PREFER_OFFLINE
	if isEnvTrue(EnvOffline) {
		c.Network.Mode = NetworkModeOffline
	} else if isEnvTrue(EnvPreferOffline) {
		c.Network.Mode = NetworkModePreferOffline
	}

	if registry := os.Getenv(EnvRegistry); registry != "" {
		c.Network.Registry = registry
	}

	if cacheDir := os.Getenv(EnvCacheDir); cacheDir != "" {
		c.Cache.Dir = cacheDir
	}

	if disabledCmds := os.Getenv(EnvDisableCommands); disabledCmds != "" {
		c.Commands.Disabled = parseCommaSeparated(disabledCmds)
	}
}

// isEnvTrue returns true if the environment variable is set to a truthy value.
func isEnvTrue(name string) bool {
	val := strings.ToLower(os.Getenv(name))
	return val == "1" || val == "true" || val == "yes"
}

// parseCommaSeparated parses a comma-separated string into a slice.
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
