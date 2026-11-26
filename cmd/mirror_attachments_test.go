package cmd

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncHTTPFileMirrorAttachments(t *testing.T) {
	syncEnv(t)
	const versionPath = "/modules/fixture/1.0/"
	const patch = "--- a/message.txt\n+++ b/message.txt\n@@ -1 +1 @@\n-before\n+after\n"
	const overlay = "from the registry\n"
	integrity := func(content string) string {
		sum := sha256.Sum256([]byte(content))
		return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
	}
	descriptor := fmt.Sprintf(`{"type":"archive","url":"file:///fixture.tar.gz","patches":{"change.patch":%q},"patch_strip":1,"overlay":{"nested/message.txt":%q},"Patches":{},"Overlay":false,"TYPE":"local_path"}`+"\n", integrity(patch), integrity(overlay))
	files := map[string]string{
		"/modules/fixture/metadata.json":           `{"versions":["1.0"]}`,
		versionPath + "MODULE.bazel":               "module(name=\"fixture\", version=\"1.0\")\n",
		versionPath + "source.json":                descriptor,
		versionPath + "patches/change.patch":       patch,
		versionPath + "overlay/nested/message.txt": overlay,
	}
	var validPatch atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == versionPath+"patches/change.patch" && !validPatch.Load() {
			_, _ = w.Write([]byte("corrupt response"))
			return
		}
		content, ok := files[req.URL.Path]
		if !ok {
			http.NotFound(w, req)
			return
		}
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(server.Close)

	destination := filepath.Join(t.TempDir(), "mirror")
	config := filepath.Join(t.TempDir(), "bz.star")
	starlark := fmt.Sprintf(`
sync.workflow(
    name = "mirror",
    origin = registry.http(%q),
    destination = registry.file(%q),
    modules = ["fixture"],
    versions = sync.all(),
)
`, server.URL, destination)
	require.NoError(t, os.WriteFile(config, []byte(starlark), 0o644))
	args := []string{"mod", "sync", "--workflow", "mirror", "--config", config}
	moduleDir := filepath.Join(destination, "modules", "fixture")
	versionDir := filepath.Join(moduleDir, "1.0")

	stdout, stderr, code := runCLI(t, args...)
	require.NotZero(t, code, "stdout=%q stderr=%q", stdout, stderr)
	require.Contains(t, stderr, "patches/change.patch: attachment integrity mismatch")
	require.NoFileExists(t, filepath.Join(versionDir, "MODULE.bazel"))
	require.NoFileExists(t, filepath.Join(moduleDir, "metadata.json"))

	validPatch.Store(true)
	stdout, stderr, code = runCLI(t, args...)
	require.Zero(t, code, "stdout=%q stderr=%q", stdout, stderr)
	for name, expected := range map[string]string{
		"source.json":                descriptor,
		"patches/change.patch":       patch,
		"overlay/nested/message.txt": overlay,
	} {
		actual, err := os.ReadFile(filepath.Join(versionDir, filepath.FromSlash(name)))
		require.NoError(t, err)
		require.Equal(t, expected, string(actual), name)
	}

	patchPath := filepath.Join(versionDir, "patches", "change.patch")
	require.NoError(t, os.WriteFile(patchPath, []byte("damaged mirror patch"), 0o644))
	validPatch.Store(false)
	stdout, stderr, code = runCLI(t, args...)
	require.NotZero(t, code, "stdout=%q stderr=%q", stdout, stderr)
	metadata, err := os.ReadFile(filepath.Join(moduleDir, "metadata.json"))
	require.NoError(t, err)
	require.JSONEq(t, `{"versions":[]}`, string(metadata))

	validPatch.Store(true)
	stdout, stderr, code = runCLI(t, args...)
	require.Zero(t, code, "stdout=%q stderr=%q", stdout, stderr)
	actual, err := os.ReadFile(patchPath)
	require.NoError(t, err)
	require.Equal(t, patch, string(actual))

	descriptorPath := filepath.Join(versionDir, "source.json")
	require.NoError(t, os.WriteFile(descriptorPath, []byte(`{"patches":{"change.patch":"sha256-invalid"}}`), 0o644))
	stdout, stderr, code = runCLI(t, args...)
	require.Zero(t, code, "stdout=%q stderr=%q", stdout, stderr)
	actual, err = os.ReadFile(descriptorPath)
	require.NoError(t, err)
	require.Equal(t, descriptor, string(actual))
}
