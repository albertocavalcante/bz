// Package cachepath defines the default cache directory.
package cachepath

import (
	"fmt"
	"os"
	"path/filepath"
)

// Resolve returns the override or the default cache directory.
func Resolve(override string) (string, error) {
	if override != "" {
		return override, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".cache", "bz"), nil
}
