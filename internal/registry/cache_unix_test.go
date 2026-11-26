//go:build unix

package registry

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheRegistryRejectsFIFO(t *testing.T) {
	dir := t.TempDir()
	moduleDir := filepath.Join(dir, "modules", "fixture")
	require.NoError(t, os.MkdirAll(moduleDir, 0o755))
	require.NoError(t, syscall.Mkfifo(filepath.Join(moduleDir, "metadata.json"), 0o600))
	_, err := NewCacheRegistry(dir).GetMetadata(context.Background(), "fixture")
	require.ErrorContains(t, err, "not a regular file")
}
