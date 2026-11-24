package publisher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain isolates every git subprocess started by this package's tests
// (helpers and the publisher itself) from the caller's git environment:
// repository-selecting variables are removed, global and system config are
// ignored, and identity and signing come from the environment. This keeps
// the tests safe when run inside a git hook, which exports GIT_DIR and
// GIT_INDEX_FILE for the repository being committed to.
func TestMain(m *testing.M) {
	os.Exit(runIsolated(m))
}

func runIsolated(m *testing.M) int {
	dir, err := os.MkdirTemp("", "bz-publisher-gitconfig-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create git config dir:", err)
		return 1
	}
	defer os.RemoveAll(dir)

	globalConfig := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(globalConfig, nil, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "create git config:", err)
		return 1
	}

	for _, name := range repoSelectingEnv {
		_ = os.Unsetenv(name)
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL":   globalConfig,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Test User",
		"GIT_AUTHOR_EMAIL":    "test@example.com",
		"GIT_COMMITTER_NAME":  "Test User",
		"GIT_COMMITTER_EMAIL": "test@example.com",
		"GIT_CONFIG_COUNT":    "3",
		"GIT_CONFIG_KEY_0":    "commit.gpgSign",
		"GIT_CONFIG_VALUE_0":  "false",
		"GIT_CONFIG_KEY_1":    "tag.gpgSign",
		"GIT_CONFIG_VALUE_1":  "false",
		"GIT_CONFIG_KEY_2":    "core.hooksPath",
		"GIT_CONFIG_VALUE_2":  filepath.Join(dir, "no-hooks"),
	} {
		_ = os.Setenv(k, v)
	}

	return m.Run()
}

// gitOutput runs git in dir with the scrubbed environment and returns stdout.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// decoyState captures everything a misdirected git command could change.
func decoyState(t *testing.T, decoy string) string {
	t.Helper()
	cfg, err := os.ReadFile(filepath.Join(decoy, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(decoy, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join([]string{
		gitOutput(t, decoy, "for-each-ref"),
		gitOutput(t, decoy, "symbolic-ref", "HEAD"),
		gitOutput(t, decoy, "status", "--porcelain", "--untracked-files=all"),
		string(cfg),
		fmt.Sprintf("index:%x", index),
	}, "\n---\n")
}

// TestGitPublisher_IgnoresAmbientGitDir runs the full publisher flow with
// GIT_DIR, GIT_WORK_TREE and GIT_INDEX_FILE pointing at a decoy repository,
// as they would inside a git hook, and checks that the decoy is untouched
// and the publish reaches the intended remote.
func TestGitPublisher_IgnoresAmbientGitDir(t *testing.T) {
	skipIfNoGit(t)
	remoteDir := setupTestGitRepo(t)

	decoy := t.TempDir()
	runGit(t, decoy, "init")
	if err := os.WriteFile(filepath.Join(decoy, "decoy.txt"), []byte("decoy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, decoy, "add", "decoy.txt")
	runGit(t, decoy, "commit", "-m", "decoy")
	before := decoyState(t, decoy)
	remoteBefore := gitOutput(t, remoteDir, "rev-parse", "main")

	t.Setenv("GIT_DIR", filepath.Join(decoy, ".git"))
	t.Setenv("GIT_WORK_TREE", decoy)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(decoy, ".git", "index"))

	pub, err := NewGitPublisher(remoteDir, "main")
	if err != nil {
		t.Fatalf("NewGitPublisher() error = %v", err)
	}
	defer pub.Close()

	ctx := context.Background()
	if err := pub.Put(ctx, "rules_go/0.50.0/MODULE.bazel", []byte("module()\n")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := pub.Finalize(ctx, "Add rules_go 0.50.0"); err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if after := decoyState(t, decoy); after != before {
		t.Errorf("decoy repository changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if remoteAfter := gitOutput(t, remoteDir, "rev-parse", "main"); remoteAfter == remoteBefore {
		t.Error("publish did not reach the intended remote")
	}
	if got := gitOutput(t, remoteDir, "log", "-1", "--format=%s", "main"); got != "Add rules_go 0.50.0" {
		t.Errorf("remote head subject = %q", got)
	}
}

func TestGitEnv_RemovesRepoSelectingVariables(t *testing.T) {
	for _, name := range repoSelectingEnv {
		t.Setenv(name, "/decoy")
	}
	t.Setenv("BZ_KEEP_ME", "1")

	env := gitEnv()
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if isRepoSelectingEnv(name) {
			t.Errorf("gitEnv kept %s", kv)
		}
	}
	found := false
	for _, kv := range env {
		if kv == "BZ_KEEP_ME=1" {
			found = true
		}
	}
	if !found {
		t.Error("gitEnv dropped an unrelated variable")
	}
}
