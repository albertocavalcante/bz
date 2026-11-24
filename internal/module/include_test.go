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

// openTree opens the workspace of a tree written by writeTree.
func openTree(t *testing.T, dir string) *Workspace {
	t.Helper()
	ws, err := OpenWorkspace(filepath.Join(dir, "MODULE.bazel"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Close() })
	return ws
}

func twoFileTree(t *testing.T) (dir string, ws *Workspace, edits []FileEdit) {
	t.Helper()
	dir = writeTree(t, map[string]string{
		"MODULE.bazel":     "module(name = \"app\")\ninclude(\"//d:x.MODULE.bazel\")\nbazel_dep(name = \"a\", version = \"1\")\n",
		"d/x.MODULE.bazel": "bazel_dep(name = \"b\", version = \"2\")\n",
	})
	ws = openTree(t, dir)
	root, _ := ws.ReadFile("MODULE.bazel")
	x, _ := ws.ReadFile("d/x.MODULE.bazel")
	edits = []FileEdit{
		{Rel: "MODULE.bazel", Original: root, Updated: append(root[:len(root):len(root)], "# 1\n"...)},
		{Rel: "d/x.MODULE.bazel", Original: x, Updated: append(x[:len(x):len(x)], "# 2\n"...)},
	}
	return dir, ws, edits
}

func assertNoTemps(t *testing.T, dir string) {
	t.Helper()
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, _ error) error {
		if d != nil && strings.HasPrefix(d.Name(), ".bz-") {
			t.Errorf("temporary file left behind: %s", p)
		}
		return nil
	})
}

func readAll(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	out := make([]string, 0, len(names))
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(n)))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	return out
}

func TestWriteAll_ReadOnlyDirectoryWritesNothing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	dir, ws, edits := twoFileTree(t)
	before := readAll(t, dir, "MODULE.bazel", "d/x.MODULE.bazel")
	if err := os.Chmod(filepath.Join(dir, "d"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "d"), 0o755) })

	err := ws.WriteAll(edits)
	if err == nil || errors.As(err, new(*PartialWriteError)) {
		t.Fatalf("want a preflight error, got %v", err)
	}
	if after := readAll(t, dir, "MODULE.bazel", "d/x.MODULE.bazel"); after[0] != before[0] || after[1] != before[1] {
		t.Errorf("files changed: %q", after)
	}
	assertNoTemps(t, dir)
}

func TestWriteAll_PartialFailureNamesWrittenAndUnwrittenFiles(t *testing.T) {
	t.Parallel()

	dir, ws, edits := twoFileTree(t)
	write := ws.writeFile
	ws.writeFile = func(rel string, data []byte) error {
		if rel == "d/x.MODULE.bazel" {
			return errors.New("disk full")
		}
		return write(rel, data)
	}

	err := ws.WriteAll(edits)
	var perr *PartialWriteError
	if !errors.As(err, &perr) {
		t.Fatalf("want *PartialWriteError, got %v", err)
	}
	if len(perr.Written) != 1 || perr.Written[0] != "MODULE.bazel" || len(perr.NotWritten) != 1 || perr.NotWritten[0] != "d/x.MODULE.bazel" {
		t.Errorf("written %v, not written %v", perr.Written, perr.NotWritten)
	}
	if !strings.Contains(err.Error(), "re-run") || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("message lacks cause or guidance: %v", err)
	}
	got := readAll(t, dir, "MODULE.bazel", "d/x.MODULE.bazel")
	if got[0] != string(edits[0].Updated) || got[1] != string(edits[1].Original) {
		t.Errorf("unexpected contents: %q", got)
	}
	assertNoTemps(t, dir)
}

func TestWriteAll_RejectsTwoEditsOfOneFile(t *testing.T) {
	t.Parallel()

	dir, ws, edits := twoFileTree(t)
	edits[1].Rel = "./MODULE.bazel"
	before := readAll(t, dir, "MODULE.bazel")
	if err := ws.WriteAll(edits); err == nil {
		t.Fatal("expected duplicate target to be rejected")
	}
	if after := readAll(t, dir, "MODULE.bazel"); after[0] != before[0] {
		t.Error("nothing may be written")
	}
}

func TestWriteAll_StaleFileWritesNothing(t *testing.T) {
	t.Parallel()

	tests := map[string]func(t *testing.T, dir string){
		"changed": func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "d", "x.MODULE.bazel"), []byte("# other\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"deleted": func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "d", "x.MODULE.bazel")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, ws, edits := twoFileTree(t)
			rootBefore := readAll(t, dir, "MODULE.bazel")[0]
			mutate(t, dir)

			err := ws.WriteAll(edits)
			if err == nil || errors.As(err, new(*PartialWriteError)) {
				t.Fatalf("want a stale-file error, got %v", err)
			}
			if name == "changed" && !errors.Is(err, ErrFileChanged) {
				t.Errorf("want ErrFileChanged, got %v", err)
			}
			if got := readAll(t, dir, "MODULE.bazel")[0]; got != rootBefore {
				t.Errorf("the first, still fresh file was written: %q", got)
			}
			assertNoTemps(t, dir)
		})
	}
}

func TestWriteAll_PreservesFileModes(t *testing.T) {
	t.Parallel()

	dir, ws, edits := twoFileTree(t)
	if err := os.Chmod(filepath.Join(dir, "MODULE.bazel"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "d", "x.MODULE.bazel"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ws.WriteAll(edits); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]os.FileMode{"MODULE.bazel": 0o640, "d/x.MODULE.bazel": 0o600} {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || info.Mode().Perm() != want {
			t.Errorf("%s mode = %v (%v), want %v", name, info.Mode().Perm(), err, want)
		}
	}
	assertNoTemps(t, dir)
}

func TestWriteAll_SymlinkedIncludedFileInsideWorkspaceKeepsLink(t *testing.T) {
	t.Parallel()

	dir := writeTree(t, map[string]string{
		"MODULE.bazel":        "module(name = \"app\")\ninclude(\"//d:x.MODULE.bazel\")\n",
		"real/x.MODULE.bazel": "bazel_dep(name = \"b\", version = \"2\")\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "d", "x.MODULE.bazel")
	if err := os.Symlink(filepath.Join("..", "real", "x.MODULE.bazel"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	ws := openTree(t, dir)
	orig, _ := ws.ReadFile("d/x.MODULE.bazel")
	if err := ws.WriteAll([]FileEdit{{Rel: "d/x.MODULE.bazel", Original: orig, Updated: []byte("# new\n")}}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link must stay a link: %v %v", info, err)
	}
	if got := readAll(t, dir, "real/x.MODULE.bazel")[0]; got != "# new\n" {
		t.Errorf("target = %q", got)
	}
	assertNoTemps(t, dir)
}

func TestWorkspace_SymlinkedRootOutsideItsDirectory(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	realDir := filepath.Join(base, "real")
	ws := filepath.Join(base, "ws")
	for _, d := range []string{realDir, filepath.Join(ws, "d")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(realDir, "MODULE.bazel")
	if err := os.WriteFile(target, []byte("module(name = \"app\")\ninclude(\"//d:x.MODULE.bazel\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "d", "x.MODULE.bazel"), []byte("bazel_dep(name = \"b\", version = \"2\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(ws, "MODULE.bazel")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	w, err := OpenWorkspace(link)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.AddDeps([]DepSpec{{Name: "c", Version: "3"}}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the root link must stay a link: %v %v", info, err)
	}
	got, _ := os.ReadFile(target)
	if !strings.HasSuffix(string(got), "bazel_dep(name = \"c\", version = \"3\")\n") {
		t.Errorf("target = %q", got)
	}
	assertNoTemps(t, base)
}

func TestWorkspace_PathWithSpacesAndNonASCII(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "my work é ü 日本")
	if err := os.MkdirAll(filepath.Join(dir, "d e"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MODULE.bazel"), []byte("module(name = \"app\")\ninclude(\"//d e:x.MODULE.bazel\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "d e", "x.MODULE.bazel"), []byte("bazel_dep(name = \"b\", version = \"2\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := openTree(t, dir)
	if got := w.File.Deps[0].Name.String(); got != "b" {
		t.Fatalf("included dependency not loaded: %s", got)
	}
	if err := w.AddDeps([]DepSpec{{Name: "c", Version: "3"}}); err != nil {
		t.Fatal(err)
	}
	orig, _ := w.ReadFile("d e/x.MODULE.bazel")
	if err := w.WriteAll([]FileEdit{{Rel: "d e/x.MODULE.bazel", Original: orig, Updated: []byte("# edited\n")}}); err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, dir, "d e/x.MODULE.bazel")[0]; got != "# edited\n" {
		t.Errorf("included file = %q", got)
	}
	assertNoTemps(t, dir)
}
