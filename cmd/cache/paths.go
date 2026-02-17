package cache

import (
	"fmt"
	"os"
	"path/filepath"
)

func resolveCacheDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".cache", "bz"), nil
}
