//go:build unix

package mod

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdate_RefusesNonregularUneditedIncludedFile(t *testing.T) {
	dir := includeWorkspace(t, map[string]string{
		"MODULE.bazel":     "module(name = \"m\")\ninclude(\"//d:x.MODULE.bazel\")\ninclude(\"//d:y.MODULE.bazel\")\n",
		"d/x.MODULE.bazel": "bazel_dep(name = \"b\", version = \"1\")\n",
		"d/y.MODULE.bazel": "bazel_dep(name = \"unrelated\", version = \"1\")\n",
	})

	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/modules/b/metadata.json" {
			http.NotFound(w, r)
			return
		}
		once.Do(func() {
			path := filepath.Join(dir, "d", "y.MODULE.bazel")
			if err := os.Remove(path); err != nil {
				t.Errorf("remove included file: %v", err)
				return
			}
			if err := syscall.Mkfifo(path, 0o644); err != nil {
				t.Errorf("replace included file with FIFO: %v", err)
			}
		})
		_, _ = w.Write([]byte(`{"versions":["1","2"]}`))
	}))
	t.Cleanup(server.Close)

	old := registryFlag
	registryFlag = server.URL
	t.Cleanup(func() { registryFlag = old })
	updateDryRun = false

	err := updateCmd.RunE(updateCmd, []string{"b"})
	require.ErrorContains(t, err, "verify d/y.MODULE.bazel: not a regular file")
	assert.Equal(t, "bazel_dep(name = \"b\", version = \"1\")\n", readRel(t, dir, "d/x.MODULE.bazel"))
}
