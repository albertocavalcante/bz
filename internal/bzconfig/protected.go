package bzconfig

import (
	"slices"
	"strings"
)

// protectedCommands can never be disabled: they are needed to discover,
// diagnose and complete bz itself.
var protectedCommands = []string{"help", "version", "completion", "__complete", "__completeNoDesc"}

// IsProtectedCommand reports whether path (a command path without the root,
// e.g. "completion bash") is or is below a command that cannot be disabled.
// The root command itself (empty path) is protected too.
func IsProtectedCommand(path string) bool {
	if path == "" {
		return true
	}
	first, _, _ := strings.Cut(path, " ")
	return slices.Contains(protectedCommands, first)
}
