package bzconfig

import (
	"fmt"
	"strings"

	"github.com/albertocavalcante/bz/internal/envutil"
)

// Environment variable names read by bz. This is the complete list; it is
// documented in docs/src/content/docs/configuration/toml.mdx.
const (
	EnvOffline         = "BZ_OFFLINE"
	EnvPreferOffline   = "BZ_PREFER_OFFLINE"
	EnvRegistry        = "BZ_REGISTRY"
	EnvCacheDir        = "BZ_CACHE_DIR"
	EnvDisableCommands = "BZ_DISABLE_COMMANDS"
)

// EnvVars returns every BZ_* variable that feeds the effective
// configuration.
func EnvVars() []string {
	return []string{EnvOffline, EnvPreferOffline, EnvRegistry, EnvCacheDir, EnvDisableCommands}
}

// EnvLayer builds the environment layer.
//
// Booleans are parsed with [envutil.ParseBool]; an unrecognized value is an
// error. Only a true value sets the mode: BZ_OFFLINE wins over
// BZ_PREFER_OFFLINE, and a false or empty value leaves the mode to lower
// layers. Empty BZ_REGISTRY, BZ_CACHE_DIR and BZ_DISABLE_COMMANDS are
// treated as unset. BZ_CACHE_DIR gets "~" expansion against home.
func EnvLayer(lookup func(string) (string, bool), home string) (Layer, error) {
	l := Layer{Kind: LayerEnv, KeyOrigins: map[Key]string{}}
	get := func(name string) string {
		v, _ := lookup(name)
		return v
	}

	offline, err := envutil.ParseBool(get(EnvOffline))
	if err != nil {
		return Layer{}, fmt.Errorf("%s: %w", EnvOffline, err)
	}
	preferOffline, err := envutil.ParseBool(get(EnvPreferOffline))
	if err != nil {
		return Layer{}, fmt.Errorf("%s: %w", EnvPreferOffline, err)
	}
	switch {
	case offline:
		m := NetworkModeOffline
		l.Settings.Mode = &m
		l.KeyOrigins[KeyMode] = EnvOffline
	case preferOffline:
		m := NetworkModePreferOffline
		l.Settings.Mode = &m
		l.KeyOrigins[KeyMode] = EnvPreferOffline
	}

	if v := strings.TrimSpace(get(EnvRegistry)); v != "" {
		l.Settings.Registry = &v
		l.KeyOrigins[KeyRegistry] = EnvRegistry
	}
	if v := strings.TrimSpace(get(EnvCacheDir)); v != "" {
		v = ExpandHome(v, home)
		l.Settings.CacheDir = &v
		l.KeyOrigins[KeyCacheDir] = EnvCacheDir
	}
	if list := parseCommaSeparated(get(EnvDisableCommands)); len(list) > 0 {
		l.Settings.Disabled = &list
		l.KeyOrigins[KeyDisabled] = EnvDisableCommands
	}
	return l, nil
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
