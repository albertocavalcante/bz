package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	server := httptest.NewServer(http.FileServer(http.Dir(fixtureOrigin(t))))
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
	if err := os.Remove(filepath.Join(dest, "modules", "rules_go", "0.50.1", "source.json")); err != nil {
		t.Fatal(err)
	}
	run()
	if _, err := os.Stat(filepath.Join(dest, "modules", "rules_go", "0.50.1", "source.json")); err != nil {
		t.Fatal(err)
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
}
