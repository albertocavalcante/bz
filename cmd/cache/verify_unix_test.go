//go:build darwin || linux

package cache

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/depgraph"
)

func TestReadCachedModuleBazel_RejectsModulesFIFOWithoutBlocking(t *testing.T) {
	for _, target := range []string{"cache", "modules", "module"} {
		t.Run(target, func(t *testing.T) {
			parent := t.TempDir()
			cacheDir := filepath.Join(parent, "cache")
			fifoPath := cacheDir
			switch target {
			case "modules":
				require.NoError(t, os.MkdirAll(cacheDir, 0o755))
				fifoPath = filepath.Join(cacheDir, "modules")
			case "module":
				fifoPath = filepath.Join(cacheDir, "modules", "rules_go", "0.50.1", "MODULE.bazel")
				require.NoError(t, os.MkdirAll(filepath.Dir(fifoPath), 0o755))
			}
			require.NoError(t, syscall.Mkfifo(fifoPath, 0o600))

			done := make(chan error, 1)
			go func() {
				_, err := readCachedModuleBazel(cacheDir, depgraph.ModuleRef{Name: "rules_go", Version: "0.50.1"})
				done <- err
			}()

			select {
			case err := <-done:
				require.Error(t, err)
			case <-time.After(time.Second):
				t.Fatal("opening a FIFO blocked waiting for a writer")
			}
		})
	}
}
