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

func runClearCmd(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	clearJSON = false
	clearForce = false
	clearCacheDir = ""
	cmd := resetClearCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), err
}

func TestClearCmd_RejectsEscapingArguments(t *testing.T) {
	parent := t.TempDir()
	cacheDir := filepath.Join(parent, "a", "cache")
	createMockCacheEntry(t, cacheDir, "rules_go", "1.0", 10)
	victim := filepath.Join(parent, "victim")
	writeFiles(t, parent, map[string]string{"victim/keep": "x"})

	for _, arg := range []string{"..", "../../victim", "", ".", "/abs", victim, "rules_go/1.0", `rules_go\..`, "Rules_go"} {
		_, err := runClearCmd(t, "", "--cache-dir="+cacheDir, "-f", arg)
		require.Error(t, err, "arg %q", arg)
		assert.Contains(t, err.Error(), "refusing to clear", "arg %q", arg)
	}
	// Valid arguments in the same invocation do not run either.
	_, err := runClearCmd(t, "", "--cache-dir="+cacheDir, "-f", "rules_go", "..")
	require.Error(t, err)

	assert.FileExists(t, filepath.Join(victim, "keep"))
	assert.DirExists(t, filepath.Join(cacheDir, "modules", "rules_go", "1.0"))
}

func TestClearCmd_JSONRequiresForce(t *testing.T) {
	cacheDir := t.TempDir()
	createMockCacheEntry(t, cacheDir, "rules_go", "1.0", 10)

	_, err := runClearCmd(t, "y\n", "--cache-dir="+cacheDir, "--json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")
	assert.DirExists(t, filepath.Join(cacheDir, "modules", "rules_go"))

	_, err = runClearCmd(t, "", "--cache-dir="+cacheDir, "--json", "--force", "rules_go")
	require.NoError(t, err)
	assert.NoDirExists(t, filepath.Join(cacheDir, "modules", "rules_go"))
}

// A module entry that is a symlink to a directory outside the cache is
// removed as a link; the target's contents are untouched.
func TestClearCmd_SymlinkedModuleDoesNotFollow(t *testing.T) {
	parent := t.TempDir()
	cacheDir := filepath.Join(parent, "cache")
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "modules"), 0o755))
	writeFiles(t, parent, map[string]string{"outside/rules_go/1.0/MODULE.bazel": "keep"})
	if err := os.Symlink(filepath.Join(parent, "outside", "rules_go"), filepath.Join(cacheDir, "modules", "rules_go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := runClearCmd(t, "", "--cache-dir="+cacheDir, "-f", "rules_go")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(parent, "outside", "rules_go", "1.0", "MODULE.bazel"))
	_, err = os.Lstat(filepath.Join(cacheDir, "modules", "rules_go"))
	assert.True(t, os.IsNotExist(err), "symlink should be removed, got %v", err)
}

// Clearing the whole cache removes only module-named directories.
func TestClearCmd_AllSkipsNonModuleEntries(t *testing.T) {
	cacheDir := t.TempDir()
	createMockCacheEntry(t, cacheDir, "rules_go", "1.0", 10)
	createMockCacheEntry(t, cacheDir, "abseil-cpp", "20240116.2", 10)
	modules := filepath.Join(cacheDir, "modules")
	writeFiles(t, modules, map[string]string{
		".git/config":       "[core]",
		"Not_A_Module/keep": "x",
		"has space/keep":    "x",
	})

	out, err := runClearCmd(t, "", "--cache-dir="+cacheDir, "--force", "--json")
	require.NoError(t, err, out)
	var result clearResult
	require.NoError(t, json.Unmarshal([]byte(out), &result), out)
	assert.Equal(t, 2, result.ModulesCleared)
	assert.ElementsMatch(t, []string{"rules_go", "abseil-cpp"}, result.Modules)
	assert.Equal(t, 3, result.Skipped)
	assert.ElementsMatch(t, []string{".git", "Not_A_Module", "has space"}, dirNames(t, modules))

	createMockCacheEntry(t, cacheDir, "rules_go", "1.0", 10)
	out, err = runClearCmd(t, "", "--cache-dir="+cacheDir, "--force")
	require.NoError(t, err, out)
	assert.Contains(t, out, "Skipping 3 entries")
	assert.FileExists(t, filepath.Join(modules, ".git", "config"))
	assert.NoDirExists(t, filepath.Join(modules, "rules_go"))
}

// A modules directory that resolves to the home directory is refused
// before anything is deleted.
func TestClearCmd_RefusesHomeDirectory(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "home")
	writeFiles(t, home, map[string]string{"projects/keep": "x", "rules_go/keep": "x"})
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cacheDir := filepath.Join(parent, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	if err := os.Symlink(home, filepath.Join(cacheDir, "modules")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	for _, args := range [][]string{{"--force"}, {"--force", "rules_go"}} {
		_, err := runClearCmd(t, "", append([]string{"--cache-dir=" + cacheDir}, args...)...)
		require.Error(t, err, args)
		assert.Contains(t, err.Error(), "home directory", args)
	}
	assert.FileExists(t, filepath.Join(home, "projects", "keep"))
	assert.FileExists(t, filepath.Join(home, "rules_go", "keep"))

	// A parent of the home directory is refused too.
	require.NoError(t, os.Remove(filepath.Join(cacheDir, "modules")))
	require.NoError(t, os.Symlink(parent, filepath.Join(cacheDir, "modules")))
	_, err := runClearCmd(t, "", "--cache-dir="+cacheDir, "--force")
	require.Error(t, err)
	assert.DirExists(t, home)
}

// checkModulesDirSafe is tested directly for the filesystem root so a
// regression can never turn into a real deletion.
func TestCheckModulesDirSafe(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	root := string(filepath.Separator)
	if v := filepath.VolumeName(os.TempDir()); v != "" {
		root = v + string(filepath.Separator)
	}
	assert.ErrorContains(t, checkModulesDirSafe(root), "filesystem root")

	link := filepath.Join(t.TempDir(), "modules")
	if err := os.Symlink(root, link); err == nil {
		assert.ErrorContains(t, checkModulesDirSafe(link), "filesystem root")
	}

	ok := filepath.Join(t.TempDir(), "modules")
	require.NoError(t, os.MkdirAll(ok, 0o755))
	assert.NoError(t, checkModulesDirSafe(ok))
}
