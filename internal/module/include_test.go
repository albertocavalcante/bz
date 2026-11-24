package module

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoad_FollowsIncludesWithProvenance(t *testing.T) {
	t.Parallel()

	dir := writeTree(t, map[string]string{
		"MODULE.bazel": `module(name = "app")
bazel_dep(name = "a", version = "1")
include("//deps:x.MODULE.bazel")
bazel_dep(name = "d", version = "4")
`,
		"deps/x.MODULE.bazel": `bazel_dep(name = "b", version = "2")
include("//deps/nested:y.MODULE.bazel")
`,
		"deps/nested/y.MODULE.bazel": `bazel_dep(name = "c", version = "3")
single_version_override(module_name = "c", version = "3")
`,
	})
	f, err := Load(filepath.Join(dir, "MODULE.bazel"))
	if err != nil {
		t.Fatal(err)
	}

	// Evaluation order: the include is spliced where it is written.
	names := make([]string, 0, len(f.Deps))
	for _, d := range f.Deps {
		names = append(names, d.Name.String())
	}
	if got := strings.Join(names, ","); got != "a,b,c,d" {
		t.Fatalf("deps = %s, want a,b,c,d", got)
	}
	if len(f.Includes) != 1 {
		t.Errorf("Includes = %d, want the root's 1", len(f.Includes))
	}

	want := []struct {
		file  string
		chain int
	}{{"MODULE.bazel", 0}, {"deps/x.MODULE.bazel", 1}, {"deps/nested/y.MODULE.bazel", 2}, {"MODULE.bazel", 0}}
	for i, d := range f.Deps {
		o := f.OriginOf(d)
		if o.File != want[i].file || len(o.Chain) != want[i].chain || strings.Contains(o.File, `\`) {
			t.Errorf("%s: origin = %+v, want file %s chain %d", d.Name, o, want[i].file, want[i].chain)
		}
	}
	if o := f.OriginOf(f.Deps[2]); o.Pos.Start.Line != 1 {
		t.Errorf("position in owning file = %d, want 1", o.Pos.Start.Line)
	}
	if len(f.Overrides) != 1 || f.OriginOf(f.Overrides[0]).File != "deps/nested/y.MODULE.bazel" {
		t.Errorf("override origin wrong: %+v", f.Overrides)
	}
}

func TestLoad_WithoutIncludeHasNoOrigins(t *testing.T) {
	t.Parallel()

	dir := writeTree(t, map[string]string{"MODULE.bazel": "module(name = \"app\")\nbazel_dep(name = \"a\", version = \"1\")\n"})
	f, err := Load(filepath.Join(dir, "MODULE.bazel"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.origins) != 0 || !f.OriginOf(f.Deps[0]).IsRoot() {
		t.Errorf("root-only file must have no included origins")
	}
}

func TestLoadContent_DoesNotFollowIncludes(t *testing.T) {
	t.Parallel()

	f, err := LoadContent("MODULE.bazel", []byte("module(name = \"app\")\ninclude(\"//deps:x.MODULE.bazel\")\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Includes) != 1 || len(f.Deps) != 0 {
		t.Errorf("LoadContent must keep includes unexpanded: %+v", f)
	}
}

func TestLoad_SymlinkCannotLeaveWorkspace(t *testing.T) {
	t.Parallel()

	outside := writeTree(t, map[string]string{"secret.MODULE.bazel": "bazel_dep(name = \"leak\", version = \"1\")\n"})
	dir := writeTree(t, map[string]string{"MODULE.bazel": "module(name = \"app\")\ninclude(\"//deps:secret.MODULE.bazel\")\n"})
	if err := os.Symlink(outside, filepath.Join(dir, "deps")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	f, err := Load(filepath.Join(dir, "MODULE.bazel"))
	if err == nil {
		t.Fatalf("expected include through an escaping symlink to fail, got deps %v", f.Deps)
	}
}

func TestWriteAll_PartialFailureNamesWrittenAndUnwrittenFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	dir := writeTree(t, map[string]string{
		"MODULE.bazel":     "module(name = \"app\")\ninclude(\"//d:x.MODULE.bazel\")\nbazel_dep(name = \"a\", version = \"1\")\n",
		"d/x.MODULE.bazel": "bazel_dep(name = \"b\", version = \"2\")\n",
	})
	ws, err := OpenWorkspace(filepath.Join(dir, "MODULE.bazel"))
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	// The files are writable but their directory is not, so the temp file for
	// the second replacement cannot be created after the first succeeded.
	if err := os.Chmod(filepath.Join(dir, "d"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "d"), 0o755) })

	root, _ := ws.ReadFile("MODULE.bazel")
	x, _ := ws.ReadFile("d/x.MODULE.bazel")
	err = ws.WriteAll([]FileEdit{
		{Rel: "MODULE.bazel", Original: root, Updated: append(root[:len(root):len(root)], '#')},
		{Rel: "d/x.MODULE.bazel", Original: x, Updated: []byte("")},
	})
	var perr *PartialWriteError
	if !errors.As(err, &perr) {
		t.Fatalf("want *PartialWriteError, got %v", err)
	}
	if len(perr.Written) != 1 || perr.Written[0] != "MODULE.bazel" || len(perr.NotWritten) != 1 || perr.NotWritten[0] != "d/x.MODULE.bazel" {
		t.Errorf("written %v, not written %v", perr.Written, perr.NotWritten)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "d", "x.MODULE.bazel")); string(got) != string(x) {
		t.Errorf("unwritten file changed: %q", got)
	}
}
