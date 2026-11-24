package mod

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/testutil"
)

// includeWorkspace writes files (slash paths) in a temp workspace, makes it the
// working directory and returns its path.
func includeWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	t.Chdir(dir)
	return dir
}

func readRel(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	require.NoError(t, err)
	return string(b)
}

func useRegistry(t *testing.T, mods map[string]testutil.TestModule) {
	t.Helper()
	old := registryFlag
	registryFlag = testutil.SetupTestRegistry(t, mods)
	t.Cleanup(func() { registryFlag = old })
	updateDryRun = false
	rmDryRun = false
}

const (
	rootWithInclude = "module(name = \"m\")\nbazel_dep(name = \"a\", version = \"1.0.0\")\ninclude(\"//d:x.MODULE.bazel\")\ninclude(\"//d:y.MODULE.bazel\")\n"
)

func TestRm_EditsOnlyTheDeclaringIncludedFile(t *testing.T) {
	x := "# keep me\r\nbazel_dep(name = \"b\", version = \"1\") # why\r\nbazel_dep(name = \"c\", version = \"2\")"
	dir := includeWorkspace(t, map[string]string{
		"MODULE.bazel":     rootWithInclude,
		"d/x.MODULE.bazel": x,
		"d/y.MODULE.bazel": "bazel_dep(name = \"e\", version = \"3\")\n",
	})
	var out bytes.Buffer
	rmCmd.SetOut(&out)
	t.Cleanup(func() { rmCmd.SetOut(os.Stdout) })

	require.NoError(t, rmCmd.RunE(rmCmd, []string{"b"}))

	assert.Equal(t, "# keep me\r\nbazel_dep(name = \"c\", version = \"2\")", readRel(t, dir, "d/x.MODULE.bazel"))
	assert.Equal(t, rootWithInclude, readRel(t, dir, "MODULE.bazel"))
	assert.Equal(t, "bazel_dep(name = \"e\", version = \"3\")\n", readRel(t, dir, "d/y.MODULE.bazel"))
	assert.Equal(t, "Removed b (from d/x.MODULE.bazel)\n", out.String())
}

func TestRm_FileIncludedTwiceIsEditedAndReportedOnce(t *testing.T) {
	dir := includeWorkspace(t, map[string]string{
		"MODULE.bazel":     "module(name = \"m\")\ninclude(\"//d:x.MODULE.bazel\")\ninclude(\"//d:x.MODULE.bazel\")\n",
		"d/x.MODULE.bazel": "bazel_dep(name = \"b\", version = \"1\")\nbazel_dep(name = \"c\", version = \"2\")\n",
	})
	var out bytes.Buffer
	rmCmd.SetOut(&out)
	t.Cleanup(func() { rmCmd.SetOut(os.Stdout) })

	require.NoError(t, rmCmd.RunE(rmCmd, []string{"b"}))

	assert.Equal(t, "bazel_dep(name = \"c\", version = \"2\")\n", readRel(t, dir, "d/x.MODULE.bazel"))
	assert.Equal(t, "Removed b (from d/x.MODULE.bazel)\n", out.String())
}

func TestRmAndUpdate_RefuseDependencyDeclaredInTwoFiles(t *testing.T) {
	useRegistry(t, map[string]testutil.TestModule{"b": {Versions: []string{"1.0.0", "2.0.0"}}})
	files := map[string]string{
		"MODULE.bazel":     "module(name = \"m\")\nbazel_dep(name = \"b\", version = \"1.0.0\")\ninclude(\"//d:x.MODULE.bazel\")\n",
		"d/x.MODULE.bazel": "bazel_dep(name = \"b\", version = \"1.0.0\")\n",
	}
	dir := includeWorkspace(t, files)

	err := rmCmd.RunE(rmCmd, []string{"b"})
	require.ErrorContains(t, err, "MODULE.bazel:2")
	require.ErrorContains(t, err, "d/x.MODULE.bazel:1")
	err = updateCmd.RunE(updateCmd, []string{"b"})
	require.ErrorContains(t, err, "declared more than once")
	err = updateCmd.RunE(updateCmd, nil)
	require.ErrorContains(t, err, "declared more than once")

	for name, want := range files {
		assert.Equal(t, want, readRel(t, dir, name), name)
	}
}

func TestRm_WarnsAboutOverrideInIncludedFile(t *testing.T) {
	dir := includeWorkspace(t, map[string]string{
		"MODULE.bazel":     "module(name = \"m\")\nbazel_dep(name = \"b\", version = \"1\")\ninclude(\"//d:x.MODULE.bazel\")\n",
		"d/x.MODULE.bazel": "single_version_override(module_name = \"b\", version = \"1\")\n",
	})
	var errOut bytes.Buffer
	rmCmd.SetErr(&errOut)
	t.Cleanup(func() { rmCmd.SetErr(os.Stderr) })

	require.NoError(t, rmCmd.RunE(rmCmd, []string{"b"}))

	assert.Contains(t, errOut.String(), "single_version_override for removed module b is still in d/x.MODULE.bazel")
	assert.Equal(t, "single_version_override(module_name = \"b\", version = \"1\")\n", readRel(t, dir, "d/x.MODULE.bazel"))
}

func TestUpdate_AllFilesAtOnce(t *testing.T) {
	useRegistry(t, map[string]testutil.TestModule{
		"a": {Versions: []string{"1.0.0", "1.1.0"}},
		"b": {Versions: []string{"1", "2"}},
		"e": {Versions: []string{"3", "4"}},
	})
	dir := includeWorkspace(t, map[string]string{
		"MODULE.bazel":     rootWithInclude,
		"d/x.MODULE.bazel": "bazel_dep(name = \"b\", version = \"1\") # c\r\n",
		"d/y.MODULE.bazel": "bazel_dep(name = \"e\", version = '3')",
	})
	var out bytes.Buffer
	updateCmd.SetOut(&out)
	t.Cleanup(func() { updateCmd.SetOut(os.Stdout) })

	require.NoError(t, updateCmd.RunE(updateCmd, nil))

	assert.Contains(t, readRel(t, dir, "MODULE.bazel"), "bazel_dep(name = \"a\", version = \"1.1.0\")")
	assert.Equal(t, "bazel_dep(name = \"b\", version = \"2\") # c\r\n", readRel(t, dir, "d/x.MODULE.bazel"))
	assert.Equal(t, "bazel_dep(name = \"e\", version = '4')", readRel(t, dir, "d/y.MODULE.bazel"))
	assert.Contains(t, out.String(), "Updated b: 1 -> 2 (major) [d/x.MODULE.bazel]")
	assert.Contains(t, out.String(), "Updated a: 1.0.0 -> 1.1.0 (minor)\n")
}

func TestUpdate_ReadOnlyIncludedFileWritesNothing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	useRegistry(t, map[string]testutil.TestModule{
		"a": {Versions: []string{"1.0.0", "1.1.0"}},
		"b": {Versions: []string{"1", "2"}},
	})
	files := map[string]string{
		"MODULE.bazel":     "module(name = \"m\")\nbazel_dep(name = \"a\", version = \"1.0.0\")\ninclude(\"//d:x.MODULE.bazel\")\n",
		"d/x.MODULE.bazel": "bazel_dep(name = \"b\", version = \"1\")\n",
	}
	dir := includeWorkspace(t, files)
	require.NoError(t, os.Chmod(filepath.Join(dir, "d", "x.MODULE.bazel"), 0o444))

	err := updateCmd.RunE(updateCmd, nil)
	require.ErrorContains(t, err, "d/x.MODULE.bazel")

	for name, want := range files {
		assert.Equal(t, want, readRel(t, dir, name), name)
	}
}

func TestUpdate_DryRunNamesFileAndWritesNothing(t *testing.T) {
	useRegistry(t, map[string]testutil.TestModule{"b": {Versions: []string{"1", "2"}}})
	files := map[string]string{
		"MODULE.bazel":     "module(name = \"m\")\ninclude(\"//d:x.MODULE.bazel\")\n",
		"d/x.MODULE.bazel": "bazel_dep(name = \"b\", version = \"1\")\n",
	}
	dir := includeWorkspace(t, files)
	var out bytes.Buffer
	updateCmd.SetOut(&out)
	t.Cleanup(func() { updateCmd.SetOut(os.Stdout); updateDryRun = false })
	updateDryRun = true

	require.NoError(t, updateCmd.RunE(updateCmd, []string{"b"}))

	assert.Contains(t, out.String(), "Would update b: 1 -> 2 (major) [d/x.MODULE.bazel]")
	for name, want := range files {
		assert.Equal(t, want, readRel(t, dir, name), name)
	}
}
