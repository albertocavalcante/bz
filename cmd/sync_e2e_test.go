package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/albertocavalcante/bz/internal/testutil"
)

// writeSyncConfig writes a bz.star that mirrors the published guide, with the
// given module list, and returns the config path and destination directory.
func writeSyncConfig(t *testing.T, origin string, modules string, extra string) (cfgPath, dest string) {
	t.Helper()
	dir := t.TempDir()
	dest = filepath.Join(dir, "mirror")
	cfg := `
bcr = registry.file(` + quote(origin) + `)
mirror = registry.file(` + quote(dest) + `)

sync.workflow(
    name = "mirror",
    origin = bcr,
    destination = mirror,
    modules = [` + modules + `],
    versions = sync.latest(count = 5),
    transforms = [
        sync.skip_yanked(),
    ],
)
` + extra
	cfgPath = filepath.Join(dir, "bz.star")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath, dest
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }

func syncEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	for _, k := range []string{"BZ_OFFLINE", "BZ_PREFER_OFFLINE", "BZ_REGISTRY", "BZ_DISABLE_COMMANDS"} {
		t.Setenv(k, "")
	}
}

func fixtureOrigin(t *testing.T) string {
	t.Helper()
	return testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.49.0", "0.50.1"}},
		"bazel_skylib": {Versions: []string{"1.7.1"}},
	})
}

func TestModSync_GuideFlowWithWorkflowFlag(t *testing.T) {
	for _, form := range [][]string{{"--workflow", "mirror"}, {"-w", "mirror"}, {"mirror"}, {"-w", "mirror", "mirror"}} {
		t.Run(strings.Join(form, " "), func(t *testing.T) {
			syncEnv(t)
			cfg, dest := writeSyncConfig(t, fixtureOrigin(t), `"rules_go", "bazel_skylib"`, "")

			args := append([]string{"mod", "sync", "-c", cfg}, form...)
			stdout, stderr, code := runCLI(t, args...)
			if code != 0 {
				t.Fatalf("exit code = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
			}
			for _, rel := range []string{
				"modules/rules_go/metadata.json",
				"modules/rules_go/0.50.1/MODULE.bazel",
				"modules/rules_go/0.50.1/source.json",
				"modules/rules_go/0.49.0/MODULE.bazel",
				"modules/bazel_skylib/metadata.json",
				"modules/bazel_skylib/1.7.1/MODULE.bazel",
			} {
				if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
					t.Errorf("missing %s under %s: %v", rel, dest, err)
				}
			}
			for _, module := range []string{"rules_go", "rules_go@0.50.1"} {
				stdout, stderr, code := runCLI(t, "mod", "info", module, "--registry", dest, "--json")
				if code != 0 || !strings.Contains(stdout, `"0.50.1"`) {
					t.Errorf("info %s: code=%d stdout=%s stderr=%s", module, code, stdout, stderr)
				}
			}
		})
	}
}

func TestModSync_WorkflowNameConflict(t *testing.T) {
	syncEnv(t)
	cfg, _ := writeSyncConfig(t, fixtureOrigin(t), `"rules_go"`, "")
	_, stderr, code := runCLI(t, "mod", "sync", "-c", cfg, "--workflow", "mirror", "other")
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr, "conflicting workflow names") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestModSync_ShorthandsDryRunAndModules(t *testing.T) {
	syncEnv(t)
	cfg, dest := writeSyncConfig(t, fixtureOrigin(t), `"rules_go", "bazel_skylib"`, "")
	stdout, stderr, code := runCLI(t, "mod", "sync", "-c", cfg, "-w", "mirror", "-n", "-m", "bazel_skylib")
	if code != 0 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Modules processed: 1") {
		t.Errorf("-m did not restrict modules:\n%s", stdout)
	}
	if entries, _ := os.ReadDir(dest); len(entries) != 0 {
		t.Errorf("dry run wrote files: %v", entries)
	}
}

func TestModSync_FailureExitsNonZeroWithSingleSummary(t *testing.T) {
	syncEnv(t)
	cfg, _ := writeSyncConfig(t, fixtureOrigin(t), `"rules_go", "no_such_module"`, "")
	stdout, stderr, code := runCLI(t, "mod", "sync", "-c", cfg, "-w", "mirror")
	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	all := stdout + stderr
	if n := strings.Count(all, "no_such_module"); n != 1 {
		t.Errorf("failing module listed %d times, want 1:\n%s", n, all)
	}
	if n := countErrorLines(stderr); n != 0 {
		t.Errorf("stderr has %d Error: lines, want 0 (already reported):\n%s", n, stderr)
	}
	if !strings.Contains(all, "Failures:") {
		t.Errorf("no failure summary:\n%s", all)
	}
}

func TestModSync_SinceFailsOnlyWhenRun(t *testing.T) {
	syncEnv(t)
	origin := fixtureOrigin(t)
	cfg, dest := writeSyncConfig(t, origin, `"rules_go"`, `
sync.workflow(
    name = "recent",
    origin = bcr,
    destination = mirror,
    modules = ["rules_go"],
    versions = sync.since("2099-01-01"),
)
`)

	stdout, stderr, code := runCLI(t, "mod", "sync", "-c", cfg, "--list")
	if code != 0 || !strings.Contains(stdout, "recent") || !strings.Contains(stdout, "mirror") {
		t.Fatalf("--list: code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}

	if _, stderr, code = runCLI(t, "mod", "sync", "-c", cfg, "-w", "mirror"); code != 0 {
		t.Fatalf("unrelated workflow failed: code=%d stderr=%s", code, stderr)
	}

	_ = os.RemoveAll(dest)
	_, stderr, code = runCLI(t, "mod", "sync", "-c", cfg, "-w", "recent")
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr, "not supported yet: release dates are not available from the registry") || !strings.Contains(stderr, `"recent"`) {
		t.Errorf("stderr = %q", stderr)
	}
	if n := countErrorLines(stderr); n != 1 {
		t.Errorf("stderr has %d Error: lines, want 1:\n%s", n, stderr)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Error("destination written although the workflow was rejected")
	}
}

func TestModSync_UnknownWorkflowListsAvailable(t *testing.T) {
	syncEnv(t)
	cfg, _ := writeSyncConfig(t, fixtureOrigin(t), `"rules_go"`, "")
	_, stderr, code := runCLI(t, "mod", "sync", "-c", cfg, "-w", "nope")
	if code == 0 || !strings.Contains(stderr, `workflow "nope" not found (available: mirror)`) {
		t.Errorf("code=%d stderr=%q", code, stderr)
	}
}

func TestModSync_HTTPToFileRegistry(t *testing.T) {
	syncEnv(t)
	var versionReads atomic.Int64
	files := http.FileServer(http.Dir(fixtureOrigin(t)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/metadata.json") {
			versionReads.Add(1)
		}
		files.ServeHTTP(w, r)
	}))
	defer server.Close()
	cfg, dest := writeSyncConfig(t, server.URL, `"rules_go"`, "")
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "bcr = registry.file(", "bcr = registry.http(", 1))
	if err := os.WriteFile(cfg, data, 0o644); err != nil {
		t.Fatal(err)
	}
	run := func() {
		t.Helper()
		out, stderr, code := runCLI(t, "mod", "sync", "-c", cfg, "--workflow", "mirror")
		if code != 0 {
			t.Fatalf("sync: %d %s %s", code, out, stderr)
		}
	}
	run()
	metadata, err := os.ReadFile(filepath.Join(dest, "modules", "rules_go", "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Versions []string `json:"versions"`
	}
	if err := json.Unmarshal(metadata, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Versions) != 2 {
		t.Fatalf("versions = %v", catalog.Versions)
	}
	for _, module := range []string{"rules_go", "rules_go@0.50.1"} {
		out, stderr, code := runCLI(t, "mod", "info", module, "--registry", dest, "--offline", "--json")
		if code != 0 || !strings.Contains(out, "0.50.1") {
			t.Fatalf("info: %d %s %s", code, out, stderr)
		}
	}
	run()
	versionDir := filepath.Join(dest, "modules", "rules_go", "0.50.1")
	modulePath, sourcePath := filepath.Join(versionDir, "MODULE.bazel"), filepath.Join(versionDir, "source.json")
	moduleBytes, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte(`{"patches":{"broken":"sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(versionDir, "patches", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(modulePath); err != nil {
		t.Fatal(err)
	}
	versionReads.Store(0)
	out, stderr, code := runCLI(t, "mod", "sync", "-c", cfg, "--workflow", "mirror", "--dry-run")
	if code != 0 || versionReads.Load() != 0 {
		t.Fatalf("incomplete core preview: %d %s %s; origin version reads = %d", code, out, stderr, versionReads.Load())
	}
	if err := os.WriteFile(modulePath, moduleBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(versionDir, "patches")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(sourcePath); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(dest, "modules", "rules_go", "metadata.json")
	if err := os.WriteFile(metadataPath, []byte(`{"versions":["0.49.0"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	versionReads.Store(0)
	out, stderr, code = runCLI(t, "mod", "sync", "-c", cfg, "--workflow", "mirror", "--dry-run")
	if code != 0 || versionReads.Load() != 0 {
		t.Fatalf("preview: %d %s %s; origin version reads = %d", code, out, stderr, versionReads.Load())
	}
	run()
	if _, err := os.Stat(filepath.Join(dest, "modules", "rules_go", "0.50.1", "source.json")); err != nil {
		t.Fatal(err)
	}
	metadata, err = os.ReadFile(metadataPath)
	if err != nil || json.Unmarshal(metadata, &catalog) != nil || !reflect.DeepEqual(catalog.Versions, []string{"0.49.0", "0.50.1"}) {
		t.Fatalf("repaired metadata = %s, %v", metadata, err)
	}
}

func TestModSync_DryRunDoesNotCreateFileDestination(t *testing.T) {
	syncEnv(t)
	cfg, dest := writeSyncConfig(t, fixtureOrigin(t), `"rules_go"`, "")
	out, stderr, code := runCLI(t, "mod", "sync", "-c", cfg, "--workflow", "mirror", "--dry-run")
	if code != 0 {
		t.Fatalf("sync: %d %s %s", code, out, stderr)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("destination exists or stat failed: %v", err)
	}
	for _, want := range []string{"Dry run complete:", "Versions planned:  2", "Versions written:  0", "Versions skipped:  0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestModSync_PreviewMixedAndApply(t *testing.T) {
	syncEnv(t)
	cfg, dest := writeSyncConfig(t, fixtureOrigin(t), `"rules_go", "bazel_skylib"`, "")
	run := func(extra ...string) string {
		t.Helper()
		args := append([]string{"mod", "sync", "-c", cfg, "--workflow", "mirror"}, extra...)
		out, stderr, code := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("sync: %d %s %s", code, out, stderr)
		}
		return out
	}
	run("--modules", "bazel_skylib")
	snapshot := func() map[string]string {
		t.Helper()
		files := map[string]string{}
		err := filepath.WalkDir(dest, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dest, p)
			if err != nil {
				return err
			}
			if d.IsDir() {
				files[rel] = "<dir>"
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files[rel] = string(data)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	out := run("--dry-run")
	for _, want := range []string{"Versions planned:  2", "Versions written:  0", "Versions skipped:  1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("preview changed destination tree")
	}
	out = run()
	for _, want := range []string{"Sync complete:", "Modules written:   2", "Modules skipped:   1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	out = run("--dry-run")
	if !strings.Contains(out, "Versions planned:  0") || !strings.Contains(out, "Versions skipped:  3") {
		t.Errorf("unexpected completed preview: %s", out)
	}
}

func TestModSync_PreviewDuplicateModules(t *testing.T) {
	syncEnv(t)
	cfg, dest := writeSyncConfig(t, fixtureOrigin(t), `"bazel_skylib", "bazel_skylib"`, "")
	out, stderr, code := runCLI(t, "mod", "sync", "-c", cfg, "--workflow", "mirror", "--dry-run")
	if code != 0 {
		t.Fatalf("sync: %d %s %s", code, out, stderr)
	}
	for _, want := range []string{"Versions planned:  1", "Versions skipped:  1", "Versions written:  0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("preview created destination: %v", err)
	}
}
