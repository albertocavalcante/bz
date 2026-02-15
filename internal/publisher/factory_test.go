package publisher

import (
	"os/exec"
	"testing"

	"github.com/albertocavalcante/bz/internal/config/modules"
)

func TestNew_FileRegistry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	reg := &modules.RegistryValue{
		Kind: modules.RegistryTypeFile,
		URL:  dir,
	}

	pub, err := New(reg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer pub.Close()

	if pub.Type() != TypeFile {
		t.Errorf("Type() = %q, want %q", pub.Type(), TypeFile)
	}
}

func TestNew_HTTPPutRegistry(t *testing.T) {
	t.Parallel()
	reg := &modules.RegistryValue{
		Kind: modules.RegistryTypeHTTPPut,
		URL:  "https://example.com/registry",
	}

	pub, err := New(reg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer pub.Close()

	if pub.Type() != TypeHTTPPut {
		t.Errorf("Type() = %q, want %q", pub.Type(), TypeHTTPPut)
	}
}

func TestNew_HTTPPutRegistryWithAuth(t *testing.T) {
	t.Parallel()
	auth := &modules.AuthValue{
		Kind:     modules.AuthTypeBasic,
		Username: "user",
		Password: "pass",
	}

	reg := &modules.RegistryValue{
		Kind: modules.RegistryTypeHTTPPut,
		URL:  "https://example.com/registry",
		Auth: auth,
	}

	pub, err := New(reg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer pub.Close()

	if pub.Type() != TypeHTTPPut {
		t.Errorf("Type() = %q, want %q", pub.Type(), TypeHTTPPut)
	}
}

func TestNew_GitRegistry(t *testing.T) {
	t.Parallel(
	// Skip if git is not available
	)

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping test")
	}

	// Set up a test git repo
	remoteDir := setupTestGitRepo(t)

	reg := &modules.RegistryValue{
		Kind:   modules.RegistryTypeGit,
		URL:    remoteDir,
		Branch: "main",
	}

	pub, err := New(reg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer pub.Close()

	if pub.Type() != TypeGit {
		t.Errorf("Type() = %q, want %q", pub.Type(), TypeGit)
	}
}

func TestNew_GitRegistryDefaultBranch(t *testing.T) {
	t.Parallel(
	// Skip if git is not available
	)

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping test")
	}

	// Set up a test git repo
	remoteDir := setupTestGitRepo(t)

	reg := &modules.RegistryValue{
		Kind: modules.RegistryTypeGit,
		URL:  remoteDir,
		// No branch specified, should default to "main"
	}

	pub, err := New(reg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer pub.Close()

	if pub.Type() != TypeGit {
		t.Errorf("Type() = %q, want %q", pub.Type(), TypeGit)
	}
}

func TestNew_HTTPRegistryNotSupported(t *testing.T) {
	t.Parallel()
	reg := &modules.RegistryValue{
		Kind: modules.RegistryTypeHTTP,
		URL:  "https://bcr.bazel.build",
	}

	_, err := New(reg)
	if err == nil {
		t.Error("New() expected error for HTTP registry (read-only)")
	}
}

func TestNew_NilRegistry(t *testing.T) {
	t.Parallel()
	_, err := New(nil)
	if err == nil {
		t.Error("New() expected error for nil registry")
	}
}

func TestNew_UnknownType(t *testing.T) {
	t.Parallel()
	reg := &modules.RegistryValue{
		Kind: "unknown",
		URL:  "https://example.com",
	}

	_, err := New(reg)
	if err == nil {
		t.Error("New() expected error for unknown registry type")
	}
}
