// Package modulearg parses common CLI module argument forms.
package modulearg

import "strings"

// Parse splits "<module>@<version>" into name and version.
//
// Scoped module names like "@scope/pkg@1.2.3" are handled correctly.
// If no version is present, version is empty.
func Parse(arg string) (name, version string) {
	lastAt := strings.LastIndex(arg, "@")
	if lastAt == -1 || lastAt == 0 {
		return arg, ""
	}

	// "@scope/pkg" has exactly one '@' and no version.
	if arg[0] == '@' && strings.Count(arg, "@") == 1 {
		return arg, ""
	}

	return arg[:lastAt], arg[lastAt+1:]
}
