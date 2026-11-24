package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const includeRoot = `module(name = "app", version = "1.0")
bazel_dep(name = "rules_go", version = "0.50.1")
include("//deps:a.MODULE.bazel")
`

// writeWorkspace writes files (slash-separated paths) under a temp dir, makes
// it the working directory and returns it.
func writeWorkspace(t *testing.T, files map[string]string) string {
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
	t.Chdir(dir)
	return dir
}

func includeWorkspace(t *testing.T) string {
	return writeWorkspace(t, map[string]string{
		"MODULE.bazel": includeRoot,
		"deps/a.MODULE.bazel": `bazel_dep(name = "gazelle", version = "0.38.0")
local_path_override(module_name = "gazelle", path = "../gz")
include("//deps/nested:b.MODULE.bazel")
`,
		"deps/nested/b.MODULE.bazel": `bazel_dep(name = "zlib", version = "1.3.1", dev_dependency = True)
`,
	})
}

func TestRun_ModListFollowsInclude(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"table", []string{"mod", "list"}, []string{"FILE", "rules_go  0.50.1", "gazelle   0.38.0", "deps/a.MODULE.bazel", "zlib      1.3.1    yes", "deps/nested/b.MODULE.bazel"}},
		{"all", []string{"mod", "list", "--all"}, []string{"gazelle   0.38.0 [deps/a.MODULE.bazel]", "[local_path] → ../gz [deps/a.MODULE.bazel]", "zlib      1.3.1 (dev) [deps/nested/b.MODULE.bazel]"}},
		{"json", []string{"mod", "list", "--json"}, []string{`"declared_in": "deps/a.MODULE.bazel"`, `"declared_in": "deps/nested/b.MODULE.bazel"`, `"name": "zlib"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			includeWorkspace(t)
			stdout, stderr, code := runCLI(t, tt.args...)
			if code != 0 {
				t.Fatalf("exit %d, stderr: %s", code, stderr)
			}
			for _, w := range tt.want {
				if !strings.Contains(stdout, w) {
					t.Errorf("stdout missing %q:\n%s", w, stdout)
				}
			}
			if strings.Contains(stdout, `"declared_in": "MODULE.bazel"`) {
				t.Errorf("root deps must not carry declared_in:\n%s", stdout)
			}
		})
	}
}

// Without include() nothing changes: no FILE column, no declared_in.
func TestRun_ModListWithoutIncludeIsUnchanged(t *testing.T) {
	writeWorkspace(t, map[string]string{"MODULE.bazel": "module(name = \"app\", version = \"1.0\")\nbazel_dep(name = \"rules_go\", version = \"0.50.1\")\n"})
	stdout, _, code := runCLI(t, "mod", "list")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if want := "app (1.0)\n\nNAME      VERSION  DEV\nrules_go  0.50.1   \n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	stdout, _, _ = runCLI(t, "mod", "list", "--json")
	if strings.Contains(stdout, "declared_in") {
		t.Errorf("json must not mention declared_in:\n%s", stdout)
	}
}

func TestRun_IncludeFailures(t *testing.T) {
	deep := map[string]string{"MODULE.bazel": "module(name = \"app\")\ninclude(\"//d:0.MODULE.bazel\")\n"}
	for i := 0; i < 20; i++ {
		deep[fmt.Sprintf("d/%d.MODULE.bazel", i)] = fmt.Sprintf("include(\"//d:%d.MODULE.bazel\")\n", i+1)
	}
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"missing file", map[string]string{"MODULE.bazel": includeRoot}, []string{"MODULE.bazel:3:1", "included file not found", "deps/a.MODULE.bazel"}},
		{"cycle", map[string]string{
			"MODULE.bazel":        includeRoot,
			"deps/a.MODULE.bazel": "include(\"//deps:a.MODULE.bazel\")\n",
		}, []string{"deps/a.MODULE.bazel:1:1", "include() cycle"}},
		{"invalid label", map[string]string{"MODULE.bazel": "module(name = \"app\")\ninclude(\"deps/a.MODULE.bazel\")\n"}, []string{"MODULE.bazel:2:1", "invalid include label"}},
		{"module in included file", map[string]string{
			"MODULE.bazel":        includeRoot,
			"deps/a.MODULE.bazel": "module(name = \"x\")\n",
		}, []string{"statement not allowed in an included file", "MODULE.bazel:3:1)", "deps/a.MODULE.bazel"}},
		{"parse error in included file", map[string]string{
			"MODULE.bazel":        includeRoot,
			"deps/a.MODULE.bazel": "bazel_dep(name = \"gazelle\"\n",
		}, []string{"deps/a.MODULE.bazel"}},
		{"limit", deep, []string{"include limit exceeded", "MaxDepth"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeWorkspace(t, tt.files)
			stdout, stderr, code := runCLI(t, "mod", "list")
			if code == 0 {
				t.Fatalf("expected non-zero exit; stdout: %s", stdout)
			}
			if n := countErrorLines(stderr); n != 1 {
				t.Errorf("%d Error: lines, want 1:\n%s", n, stderr)
			}
			if strings.Contains(stdout, "rules_go") {
				t.Errorf("partial result must not be printed:\n%s", stdout)
			}
			for _, w := range tt.want {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr missing %q:\n%s", w, stderr)
				}
			}
		})
	}
}

func TestRun_EditingGuardsForIncludedDeps(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"add duplicate", []string{"mod", "add", "--no-verify", "zlib@1.0"}, "zlib is declared in deps/nested/b.MODULE.bazel:1; editing included files is not supported yet"},
		{"rm", []string{"mod", "rm", "gazelle"}, "gazelle is declared in deps/a.MODULE.bazel:1; editing included files is not supported yet"},
		{"update", []string{"mod", "update", "gazelle"}, "gazelle is declared in deps/a.MODULE.bazel:1; editing included files is not supported yet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := includeWorkspace(t)
			before := mustRead(t, filepath.Join(dir, "MODULE.bazel"))
			_, stderr, code := runCLI(t, tt.args...)
			if code == 0 {
				t.Fatal("expected non-zero exit")
			}
			if n := countErrorLines(stderr); n != 1 || !strings.Contains(stderr, tt.want) {
				t.Errorf("stderr = %q, want one Error: line containing %q", stderr, tt.want)
			}
			if after := mustRead(t, filepath.Join(dir, "MODULE.bazel")); after != before {
				t.Errorf("MODULE.bazel was modified:\n%s", after)
			}
		})
	}
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
