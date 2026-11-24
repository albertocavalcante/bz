package publisher

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/albertocavalcante/bz/internal/coord"
)

// hostilePaths are publisher paths that must be refused before any write or
// request. They model coordinates taken from untrusted registry metadata.
var hostilePaths = []string{
	"rules_go/../../../esc/MODULE.bazel",
	"rules_go/../../esc/source.json",
	"../esc/MODULE.bazel",
	"../../esc/MODULE.bazel",
	"/abs/esc/MODULE.bazel",
	"rules_go/..",
	"..",
	"",
	"rules_go",
	`rules_go\..\..\esc\MODULE.bazel`,
	"rules_go/%2e%2e/MODULE.bazel",
	"rules_go//MODULE.bazel",
	".git/hooks/pre-commit",
}

// assertNoFilesUnder fails if any regular file or symlink exists under dir,
// other than those in allowed (paths relative to dir).
func assertNoFilesUnder(t *testing.T, dir string, allowed ...string) {
	t.Helper()
	ok := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		ok[filepath.FromSlash(a)] = true
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if !ok[rel] {
			t.Errorf("unexpected file outside the publisher base: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFilePublisher_RejectsEscapingPaths(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	base := filepath.Join(parent, "dest", "modules")
	pub, err := NewFilePublisher(base)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()
	ctx := context.Background()

	for _, p := range hostilePaths {
		if err := pub.Put(ctx, p, []byte("x")); !errors.Is(err, coord.ErrInvalidPath) {
			t.Errorf("Put(%q) error = %v, want ErrInvalidPath", p, err)
		}
		if _, err := pub.Exists(ctx, p); !errors.Is(err, coord.ErrInvalidPath) {
			t.Errorf("Exists(%q) error = %v, want ErrInvalidPath", p, err)
		}
	}

	assertNoFilesUnder(t, parent)
}

// TestFilePublisher_SymlinkInsideBase checks the second layer: even with a
// valid path, a symlink planted inside the base cannot redirect a write
// outside it.
func TestFilePublisher_SymlinkInsideBase(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	base := filepath.Join(parent, "dest")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	pub, err := NewFilePublisher(base)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()
	if err := os.Symlink(outside, filepath.Join(base, "rules_go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err = pub.Put(context.Background(), "rules_go/1.0/MODULE.bazel", []byte("x"))
	if err == nil {
		t.Fatal("Put through a symlink that leaves the base succeeded")
	}
	assertNoFilesUnder(t, outside)
}

func TestGitPublisher_RejectsEscapingPaths(t *testing.T) {
	skipIfNoGit(t)
	remoteDir := setupTestGitRepo(t)

	// Keep the clone inside a test-owned directory so a regression could
	// only ever write inside t.TempDir().
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	pub, err := NewGitPublisher(remoteDir, "main")
	if err != nil {
		t.Fatalf("NewGitPublisher() error = %v", err)
	}
	defer pub.Close()
	if rel, err := filepath.Rel(tmp, pub.workDir); err != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("work dir %q not under %q", pub.workDir, tmp)
	}
	ctx := context.Background()

	for _, p := range hostilePaths {
		if err := pub.Put(ctx, p, []byte("x")); !errors.Is(err, coord.ErrInvalidPath) {
			t.Errorf("Put(%q) error = %v, want ErrInvalidPath", p, err)
		}
		if _, err := pub.Exists(ctx, p); !errors.Is(err, coord.ErrInvalidPath) {
			t.Errorf("Exists(%q) error = %v, want ErrInvalidPath", p, err)
		}
	}
	if pub.hasChanges {
		t.Error("rejected writes marked the clone as changed")
	}

	// Everything outside the clone's own directory must be untouched.
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Join(tmp, e.Name()) != pub.workDir {
			t.Errorf("unexpected entry next to the clone: %s", e.Name())
		}
	}
}

func TestHTTPPublisher_RejectsEscapingPaths(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.RequestURI)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	pub, err := NewHTTPPublisher(server.URL+"/registry", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, p := range hostilePaths {
		if err := pub.Put(ctx, p, []byte("x")); !errors.Is(err, coord.ErrInvalidPath) {
			t.Errorf("Put(%q) error = %v, want ErrInvalidPath", p, err)
		}
		if _, err := pub.Exists(ctx, p); !errors.Is(err, coord.ErrInvalidPath) {
			t.Errorf("Exists(%q) error = %v, want ErrInvalidPath", p, err)
		}
	}

	mu.Lock()
	if len(requests) != 0 {
		t.Errorf("sent %d request(s) for invalid paths: %v", len(requests), requests)
	}
	requests = nil
	mu.Unlock()

	// A valid path only ever addresses locations under the base URL.
	if err := pub.Put(ctx, "rules_go/0.50.0/MODULE.bazel", []byte("x")); err != nil {
		t.Fatalf("Put(valid) error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, r := range requests {
		if strings.Contains(r, "..") || !strings.Contains(r, " /registry/rules_go") {
			t.Errorf("request outside the base URL: %s", r)
		}
	}
}
