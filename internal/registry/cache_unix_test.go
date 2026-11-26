//go:build darwin || linux

package registry

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheRegistryRejectsFIFO(t *testing.T) {
	dir := t.TempDir()
	moduleDir := filepath.Join(dir, "modules", "fixture")
	require.NoError(t, os.MkdirAll(moduleDir, 0o755))
	require.NoError(t, syscall.Mkfifo(filepath.Join(moduleDir, "metadata.json"), 0o600))
	done := make(chan error, 1)
	go func() {
		_, err := NewCacheRegistry(dir).GetMetadata(context.Background(), "fixture")
		done <- err
	}()
	select {
	case err := <-done:
		require.ErrorContains(t, err, "not a regular file")
	case <-time.After(time.Second):
		t.Fatal("opening a FIFO blocked waiting for a writer")
	}
}
