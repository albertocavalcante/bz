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
	"rules_go/1.0/%2e%2e/%2e%2e/pwn",
	"rules_go/1.0/x?y=1#frag",
	"rules_go/1.0/a\nb",
	"rules_go/1.0/.git/config",
	"rules_go/1.0/.GIT/hooks/pre-commit",
	"rules_go/1.0/x:ads",
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
	mu.Unlock()
	assertNoSpecialRequests(t, &mu, &requests)
	mu.Lock()
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

// assertNoSpecialRequests fails if any recorded request URI carries "..",
// an encoded dot, a query or a fragment.
func assertNoSpecialRequests(t *testing.T, mu *sync.Mutex, requests *[]string) {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	for _, r := range *requests {
		lower := strings.ToLower(r)
		if strings.Contains(r, "..") || strings.Contains(lower, "%2e") || strings.ContainsAny(r, "?#") {
			t.Errorf("request carries path-special content: %s", r)
		}
	}
}

// TestHTTPPublisher_EscapesSegments exercises the second layer directly:
// even a path that bypassed validation is sent as literal, escaped segments,
// so the server never sees a decoded "..", a query or a fragment.
func TestHTTPPublisher_EscapesSegments(t *testing.T) {
	t.Parallel()

	type seen struct{ path, rawPath, rawQuery, uri string }
	var mu sync.Mutex
	var got []seen
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, seen{r.URL.Path, r.URL.RawPath, r.URL.RawQuery, r.RequestURI})
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	pub, err := NewHTTPPublisher(server.URL+"/registry", nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, raw := range []string{"rules_go/1.0/..", "rules_go/./x", "rules_go//x"} {
		if _, err := pub.joinURL(raw); !errors.Is(err, coord.ErrInvalidPath) {
			t.Errorf("joinURL(%q) error = %v, want ErrInvalidPath", raw, err)
		}
	}

	for _, raw := range []string{
		"rules_go/1.0/%2e%2e/%2e%2e/pwn",
		"rules_go/1.0/x?y=1#frag",
		"rules_go/1.0/a b",
	} {
		u, err := pub.joinURL(raw)
		if err != nil {
			t.Fatalf("joinURL(%q): %v", raw, err)
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, u, strings.NewReader("x"))
		if err != nil {
			t.Fatalf("NewRequest(%q): %v", u, err)
		}
		resp, err := pub.httpClient.Do(req)
		if err != nil {
			t.Fatalf("PUT %q: %v", u, err)
		}
		_ = resp.Body.Close()
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 3 {
		t.Fatalf("got %d requests, want 3: %+v", len(got), got)
	}
	for _, g := range got {
		if g.rawQuery != "" || strings.Contains(g.uri, "#") {
			t.Errorf("request has a query or fragment: %+v", g)
		}
		// On the wire, special characters are escaped: no raw "..", encoded
		// dot, query or fragment reaches the server.
		if strings.Contains(g.uri, "..") || strings.Contains(strings.ToLower(g.uri), "%2e") || strings.ContainsAny(g.uri, "?#") {
			t.Errorf("request URI carries path-special content: %+v", g)
		}
		if !strings.HasPrefix(g.path, "/registry/rules_go/1.0/") {
			t.Errorf("request left the base path: %+v", g)
		}
		for _, seg := range strings.Split(g.path, "/") {
			if seg == ".." {
				t.Errorf("decoded path has a '..' segment: %+v", g)
			}
		}
	}
}
