package builtins

import (
	"fmt"
	"os"
	"strings"

	"go.starlark.net/starlark"
)

// Env is a Starlark builtin that retrieves environment variables.
//
// Usage in Starlark:
//
//	token = env("REGISTRY_TOKEN")
//	token = env("REGISTRY_TOKEN", default="")
var Env = starlark.NewBuiltin("env", envImpl)

// envAllowedPrefixesKey is the thread-local key for storing allowed env var prefixes.
const envAllowedPrefixesKey = "env_allowed_prefixes"

// SetEnvAllowedPrefixes sets the allowed environment variable prefixes for security.
// If set, only environment variables with one of these prefixes can be accessed.
// Pass nil or empty slice to allow all environment variables.
func SetEnvAllowedPrefixes(thread *starlark.Thread, prefixes []string) {
	thread.SetLocal(envAllowedPrefixesKey, prefixes)
}

// GetEnvAllowedPrefixes retrieves the allowed prefixes from a Starlark thread.
func GetEnvAllowedPrefixes(thread *starlark.Thread) []string {
	if v := thread.Local(envAllowedPrefixesKey); v != nil {
		if prefixes, ok := v.([]string); ok {
			return prefixes
		}
	}
	return nil
}

// isAllowedEnvVar checks if an environment variable name is allowed.
func isAllowedEnvVar(name string, allowedPrefixes []string) bool {
	if len(allowedPrefixes) == 0 {
		return true
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func envImpl(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var defaultValue starlark.Value

	if err := starlark.UnpackArgs("env", args, kwargs,
		"name", &name,
		"default?", &defaultValue,
	); err != nil {
		return nil, err
	}

	// Check if the environment variable is allowed
	allowedPrefixes := GetEnvAllowedPrefixes(thread)
	if !isAllowedEnvVar(name, allowedPrefixes) {
		return nil, fmt.Errorf("env: access to %q is not allowed (restricted by prefix policy)", name)
	}

	// Look up the environment variable
	value, exists := os.LookupEnv(name)

	if !exists {
		if defaultValue != nil {
			return defaultValue, nil
		}
		return nil, fmt.Errorf("env: %q is not set and no default provided", name)
	}

	return starlark.String(value), nil
}
