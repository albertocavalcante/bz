package modsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/albertocavalcante/bz/internal/config"
	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestService_Run_GitDestinationBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	// Isolate both fixture commands and the publisher subprocesses from Git
	// variables inherited from a shell or hook.
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "GIT_CONFIG") || slices.Contains([]string{
			"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
			"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_NAMESPACE", "GIT_PREFIX",
		}, upper) {
			value := os.Getenv(name)
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Setenv(name, value) })
		}
	}
	configDir := t.TempDir()
	globalConfig := filepath.Join(configDir, "gitconfig")
	if err := os.WriteFile(globalConfig, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GIT_CONFIG_COUNT", "3")
	t.Setenv("GIT_CONFIG_KEY_0", "commit.gpgSign")
	t.Setenv("GIT_CONFIG_VALUE_0", "false")
	t.Setenv("GIT_CONFIG_KEY_1", "tag.gpgSign")
	t.Setenv("GIT_CONFIG_VALUE_1", "false")
	t.Setenv("GIT_CONFIG_KEY_2", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_2", filepath.Join(configDir, "no-hooks"))
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	git := func(t *testing.T, dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	for _, tc := range []struct{ name, branch, target string }{{"default", "", "main"}, {"named", "release", "release"}} {
		t.Run(tc.name, func(t *testing.T) {
			remote, work := t.TempDir(), t.TempDir()
			git(t, remote, "init", "--bare")
			git(t, work, "init")
			for _, branch := range []string{"decoy", "main", "release"} {
				git(t, work, "checkout", "--orphan", branch)
				git(t, work, "commit", "--allow-empty", "-m", branch)
				git(t, work, "push", remote, branch)
			}
			git(t, remote, "symbolic-ref", "HEAD", "refs/heads/decoy")
			refs := func() string { return git(t, remote, "show-ref", "--heads") }
			before := refs()
			origin := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{"rules_go": {Versions: []string{"0.50.0"}}})
			workflow := &config.Workflow{Name: "sync", Origin: &config.Registry{Type: "file", URL: origin}, Destination: &config.Registry{Type: "git", URL: remote, Branch: tc.branch}, Modules: []string{"rules_go"}}
			svc := NewService(&config.Config{Workflows: []*config.Workflow{workflow}})
			result, err := svc.Run(t.Context(), "sync", Options{DryRun: true})
			if err != nil || len(result.Errors) != 0 || result.VersionsPlanned != 1 {
				t.Fatalf("preview = %+v, %v", result, err)
			}
			if got := refs(); got != before {
				t.Fatalf("preview changed refs: before %s after %s", before, got)
			}
			result, err = svc.Run(t.Context(), "sync", Options{})
			if err != nil || len(result.Errors) != 0 || result.ModulesWritten != 1 {
				t.Fatalf("sync = %+v, %v", result, err)
			}
			for _, branch := range []string{"decoy", "main", "release"} {
				old := git(t, remote, "rev-parse", branch+"^{}")
				var prior string
				for _, line := range strings.Split(before, "\n") {
					fields := strings.Fields(line)
					if len(fields) == 2 && fields[1] == "refs/heads/"+branch {
						prior = fields[0]
					}
				}
				if (old != prior) != (branch == tc.target) {
					t.Errorf("branch %s changed = %t, want %t", branch, old != prior, branch == tc.target)
				}
			}
			git(t, remote, "show", tc.target+":rules_go/0.50.0/MODULE.bazel")
		})
	}
}
