package bzconfig

import (
	"fmt"
	"strings"

	"github.com/albertocavalcante/bz/internal/envutil"
)

// Environment variable names read by bz.
const (
	EnvOffline         = "BZ_OFFLINE"
	EnvPreferOffline   = "BZ_PREFER_OFFLINE"
	EnvRegistry        = "BZ_REGISTRY"
	EnvCacheDir        = "BZ_CACHE_DIR"
	EnvDisableCommands = "BZ_DISABLE_COMMANDS"
)

// EnvLayer builds the environment layer.
//
// Booleans are parsed with [envutil.ParseBool]. An unrecognized value is
// treated as false and reported in the returned error; the layer is usable
// either way. Only a true value sets the mode: BZ_OFFLINE wins over
// BZ_PREFER_OFFLINE, and a false or empty value leaves the mode to lower
// layers. Empty (or all-space) BZ_REGISTRY and BZ_CACHE_DIR are unset. Any
// non-empty BZ_DISABLE_COMMANDS replaces the list, so "," clears it.
// BZ_CACHE_DIR gets "~" expansion against home.
func EnvLayer(lookup func(string) (string, bool), home string) (Layer, error) {
	l := Layer{Kind: LayerEnv, KeyOrigins: map[Key]string{}}
	get := func(name string) string {
		v, _ := lookup(name)
		return v
	}

	var errs []error
	offline, err := envutil.ParseBool(get(EnvOffline))
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvOffline, err))
	}
	preferOffline, err := envutil.ParseBool(get(EnvPreferOffline))
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvPreferOffline, err))
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
	if v := get(EnvDisableCommands); v != "" {
		list := parseCommaSeparated(v)
		l.Settings.Disabled = &list
		l.KeyOrigins[KeyDisabled] = EnvDisableCommands
	}
	return l, joinProblems(errs)
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
