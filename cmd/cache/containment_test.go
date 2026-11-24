package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/registry"
)

// writeFiles writes rel -> content under base.
func writeFiles(t *testing.T, base string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(base, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
}

// dirNames lists the entry names of dir.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// setupHostileCacheRegistry builds a file registry by hand (testutil would
// itself follow the traversal) whose metadata and module listing carry
// invalid coordinates. Readable files sit where a traversal would resolve, so
// only validation stops the download.
func setupHostileCacheRegistry(t *testing.T, parent string) string {
	t.Helper()
	writeFiles(t, parent, map[string]string{
		"reg/modules/rules_go/metadata.json":    `{"versions":["1.0","../../../esc"]}`,
		"reg/modules/rules_go/1.0/MODULE.bazel": `module(name = "rules_go", version = "1.0")`,
		"reg/modules/gazelle/metadata.json":     `{"versions":["2.0"]}`,
		"reg/modules/gazelle/2.0/MODULE.bazel":  `module(name = "gazelle", version = "2.0")`,
		"reg/modules/Evil/metadata.json":        `{"versions":["1.0"]}`,
		"reg/modules/Evil/1.0/MODULE.bazel":     `module(name = "Evil", version = "1.0")`,
		"esc/MODULE.bazel":                      "hostile",
		"esc/source.json":                       "{}",
	})
	return filepath.Join(parent, "reg")
}

func runDownloadCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	downloadAll = false
	downloadJSON = false
	downloadRegistry = ""
	downloadCacheDir = ""
	cmd := resetDownloadCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), err
}

func TestDownloadCmd_AllHostileRegistryStaysInCache(t *testing.T) {
	parent := t.TempDir()
	reg := setupHostileCacheRegistry(t, parent)
	// From cache/modules/rules_go/<v>, three ".." levels reach box/.
	box := filepath.Join(parent, "box")
	cacheDir := filepath.Join(box, "cache")

	out, err := runDownloadCmd(t, "--all", "--json", "--registry="+reg, "--cache-dir="+cacheDir)
	require.NoError(t, err, out) // --json reports failures in the payload

	var result downloadResult
	require.NoError(t, json.Unmarshal([]byte(out), &result), out)

	assert.Contains(t, result.Modules, "rules_go@1.0")
	assert.Contains(t, result.Modules, "gazelle@2.0")
	assert.Equal(t, 2, result.Failed, "errors: %v", result.Errors)
	joined := strings.Join(result.Errors, "\n")
	assert.Contains(t, joined, "invalid module version")
	assert.Contains(t, joined, "invalid module name")

	assert.Equal(t, []string{"cache"}, dirNames(t, box), "nothing may be written next to the cache")
	assert.ElementsMatch(t, []string{"gazelle", "rules_go"}, dirNames(t, filepath.Join(cacheDir, "modules")))
	assert.ElementsMatch(t, []string{"1.0", "metadata.json"}, dirNames(t, filepath.Join(cacheDir, "modules", "rules_go")))
}

// downloadModule is the last line of defence for coordinates that reach it
// from MODULE.bazel files or dependency resolution.
func TestDownloadModule_RejectsInvalidCoordinates(t *testing.T) {
	parent := t.TempDir()
	reg, err := registry.New(setupHostileCacheRegistry(t, parent))
	require.NoError(t, err)
	box := filepath.Join(parent, "box")
	cacheDir := filepath.Join(box, "cache")

	cases := []struct{ name, version string }{
		{"rules_go", "../../../esc"},
		{"rules_go", ".."},
		{"rules_go", ""},
		{"../../esc", "1.0"},
		{"Evil", "1.0"},
	}
	for _, c := range cases {
		err := downloadModule(context.Background(), reg, cacheDir, c.name, c.version)
		require.Error(t, err, "%s@%s", c.name, c.version)
		assert.True(t, errors.Is(err, coord.ErrInvalidModuleName) || errors.Is(err, coord.ErrInvalidVersion),
			"%s@%s: %v", c.name, c.version, err)
	}
	assert.NoDirExists(t, box, "rejected coordinates must not touch the filesystem")
}

func TestDownloadCmd_RejectsInvalidArguments(t *testing.T) {
	parent := t.TempDir()
	reg := setupHostileCacheRegistry(t, parent)
	cacheDir := filepath.Join(parent, "cache")

	for _, arg := range []string{"../../x", "../../x@1.0", "rules_go@../../../esc", "rules_go@..", "/abs@1.0", "Evil"} {
		_, err := runDownloadCmd(t, "--registry="+reg, "--cache-dir="+cacheDir, arg)
		require.Error(t, err, arg)
		assert.Contains(t, err.Error(), "invalid module argument", arg)
	}
	assert.NoDirExists(t, cacheDir, "rejected arguments must not create the cache")
}
