package publisher

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// skipIfNoGit skips the test if git is not available.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping test")
	}
}

// setupTestGitRepo creates a temporary git repository for testing.
func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	skipIfNoGit(t)

	// Create a bare repository to act as remote
	remoteDir := t.TempDir()
	runGit(t, remoteDir, "init", "--bare")

	// Create a working repository and push initial commit
	workDir := t.TempDir()
	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.email", "test@example.com")
	runGit(t, workDir, "config", "user.name", "Test User")

	// Create initial file and commit
	readmePath := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workDir, "add", ".")
	runGit(t, workDir, "commit", "-m", "Initial commit")
	runGit(t, workDir, "branch", "-M", "main")
	runGit(t, workDir, "remote", "add", "origin", remoteDir)
	runGit(t, workDir, "push", "-u", "origin", "main")

	return remoteDir
}

// runGit runs a git command in the specified directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func TestGitPublisher_Type(t *testing.T) {
	skipIfNoGit(t)
	remoteDir := setupTestGitRepo(t)

	pub, err := NewGitPublisher(remoteDir, "main")
	if err != nil {
		t.Fatalf("NewGitPublisher() error = %v", err)
	}
	defer pub.Close()

	if got := pub.Type(); got != TypeGit {
		t.Errorf("Type() = %q, want %q", got, TypeGit)
	}
}

func TestGitPublisher_Put(t *testing.T) {
	skipIfNoGit(t)
	remoteDir := setupTestGitRepo(t)

	pub, err := NewGitPublisher(remoteDir, "main")
	if err != nil {
		t.Fatalf("NewGitPublisher() error = %v", err)
	}
	defer pub.Close()

	ctx := context.Background()
	content := []byte(`module(name = "rules_go", version = "0.50.0")`)

	// Put a file
	err = pub.Put(ctx, "rules_go/0.50.0/MODULE.bazel", content)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Verify file was created in work directory
	fullPath := filepath.Join(pub.workDir, "rules_go/0.50.0/MODULE.bazel")
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("file content = %q, want %q", string(data), string(content))
	}
}

func TestGitPublisher_Exists(t *testing.T) {
	skipIfNoGit(t)
	remoteDir := setupTestGitRepo(t)

	pub, err := NewGitPublisher(remoteDir, "main")
	if err != nil {
		t.Fatalf("NewGitPublisher() error = %v", err)
	}
	defer pub.Close()

	ctx := context.Background()

	// File doesn't exist yet
	exists, err := pub.Exists(ctx, "rules_go/0.50.0/MODULE.bazel")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}

	// README.md should exist (from initial commit)
	exists, err = pub.Exists(ctx, "README.md")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true for README.md")
	}
}

func TestGitPublisher_Finalize(t *testing.T) {
	skipIfNoGit(t)
	remoteDir := setupTestGitRepo(t)

	pub, err := NewGitPublisher(remoteDir, "main")
	if err != nil {
		t.Fatalf("NewGitPublisher() error = %v", err)
	}
	defer pub.Close()

	// Configure git user in work directory
	runGit(t, pub.workDir, "config", "user.email", "test@example.com")
	runGit(t, pub.workDir, "config", "user.name", "Test User")

	ctx := context.Background()

	// Put a file
	content := []byte(`module(name = "rules_go", version = "0.50.0")`)
	err = pub.Put(ctx, "rules_go/0.50.0/MODULE.bazel", content)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Finalize (commit and push)
	err = pub.Finalize(ctx, "Add rules_go module")
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	// Verify the commit was pushed by cloning fresh
	verifyDir := t.TempDir()
	runGit(t, verifyDir, "clone", remoteDir, ".")

	// Check that the file exists in the new clone
	fullPath := filepath.Join(verifyDir, "rules_go/0.50.0/MODULE.bazel")
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v (file not pushed?)", err)
	}

	if string(data) != string(content) {
		t.Errorf("file content = %q, want %q", string(data), string(content))
	}
}

func TestGitPublisher_Finalize_NoChanges(t *testing.T) {
	skipIfNoGit(t)
	remoteDir := setupTestGitRepo(t)

	pub, err := NewGitPublisher(remoteDir, "main")
	if err != nil {
		t.Fatalf("NewGitPublisher() error = %v", err)
	}
	defer pub.Close()

	ctx := context.Background()

	// Finalize without any changes should be a no-op
	err = pub.Finalize(ctx, "Empty commit")
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
}

func TestGitPublisher_Close(t *testing.T) {
	skipIfNoGit(t)
	remoteDir := setupTestGitRepo(t)

	pub, err := NewGitPublisher(remoteDir, "main")
	if err != nil {
		t.Fatalf("NewGitPublisher() error = %v", err)
	}

	workDir := pub.workDir

	// Verify work directory exists
	if _, err := os.Stat(workDir); err != nil {
		t.Fatalf("work directory doesn't exist: %v", err)
	}

	// Close should remove the work directory
	err = pub.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify work directory was removed
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Error("Close() did not remove work directory")
	}
}

func TestGitPublisher_DefaultBranch(t *testing.T) {
	skipIfNoGit(t)
	remoteDir := setupTestGitRepo(t)

	// Empty branch should default to "main"
	pub, err := NewGitPublisher(remoteDir, "")
	if err != nil {
		t.Fatalf("NewGitPublisher() error = %v", err)
	}
	defer pub.Close()

	if pub.branch != "main" {
		t.Errorf("branch = %q, want %q", pub.branch, "main")
	}
}

func TestGitPublisher_EmptyURL(t *testing.T) {
	_, err := NewGitPublisher("", "main")
	if err == nil {
		t.Error("NewGitPublisher() expected error for empty URL")
	}
}

func TestGitPublisher_InvalidRepo(t *testing.T) {
	skipIfNoGit(t)

	// Try to clone a non-existent repo
	_, err := NewGitPublisher("/nonexistent/repo", "main")
	if err == nil {
		t.Error("NewGitPublisher() expected error for invalid repo")
	}
}
