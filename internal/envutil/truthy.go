// Package envutil provides shared environment parsing helpers.
package envutil

import (
	"os"
	"strings"
)

// IsTruthy reports whether a value is an accepted truthy token.
func IsTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

// IsTruthyEnv reports whether an environment variable is set to a truthy token.
func IsTruthyEnv(name string) bool {
	return IsTruthy(os.Getenv(name))
}
