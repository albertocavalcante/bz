// Package envutil provides shared environment parsing helpers.
package envutil

import (
	"fmt"
	"os"
	"strings"
)

// ParseBool parses a boolean environment value.
//
// Accepted values (case-insensitive, surrounding spaces ignored):
//   - true: "1", "true", "yes", "on"
//   - false: "", "0", "false", "no", "off"
//
// Any other value is an error, so a typo such as BZ_OFFLINE=ture is reported
// instead of being read as false.
func ParseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "", "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q (use 1, true, yes, on or 0, false, no, off)", value)
	}
}

// IsTruthy reports whether a value is an accepted truthy token.
// Invalid values are treated as false; use ParseBool to detect them.
func IsTruthy(value string) bool {
	v, err := ParseBool(value)
	return err == nil && v
}

// IsTruthyEnv reports whether an environment variable is set to a truthy token.
func IsTruthyEnv(name string) bool {
	return IsTruthy(os.Getenv(name))
}
