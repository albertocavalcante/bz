package modsync

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/albertocavalcante/bz/internal/config"
	"github.com/albertocavalcante/bz/internal/coord"
)

// setupHostileRegistry builds a file registry by hand whose metadata lists a
// traversal version next to a valid one. It also places readable files where
// the traversal would resolve, so only coordinate validation stops the read.
func setupHostileRegistry(t *testing.T, parent string) string {
	t.Helper()
	reg := filepath.Join(parent, "reg")
	files := map[string]string{
		"reg/modules/rules_go/metadata.json":    `{"versions":["1.0","../../../ESCAPED"]}`,
		"reg/modules/rules_go/1.0/MODULE.bazel": `module(name = "rules_go", version = "1.0")`,
		"reg/modules/rules_go/1.0/source.json":  `{"url":"https://example.com/a.tar.gz"}`,
		"ESCAPED/MODULE.bazel":                  "hostile",
		"ESCAPED/source.json":                   `{"url":"https://example.com/b.tar.gz"}`,
	}
	for rel, content := range files {
		p := filepath.Join(parent, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return reg
}

func hostileWorkflow(reg string, dest *config.Registry) *config.Config {
	return &config.Config{Workflows: []*config.Workflow{{
		Name:        "hostile",
		Origin:      &config.Registry{Type: "file", URL: reg},
		Destination: dest,
		Modules:     []string{"rules_go"},
		Versions:    &config.VersionSelector{Type: "all"},
	}}}
}

func TestRunner_HostileVersionFileDestination(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	reg := setupHostileRegistry(t, parent)
	// A traversal of three levels from dest/rules_go/<v> lands in parent/x.
	dest := filepath.Join(parent, "x", "y", "dest")

	result, err := NewService(hostileWorkflow(reg, &config.Registry{Type: "file", URL: dest})).
		Run(context.Background(), "hostile", Options{})
	if err != nil {
		t.Fatal(err)
	}

	if result.ModulesWritten != 1 {
		t.Errorf("ModulesWritten = %d, want 1 (the valid version)", result.ModulesWritten)
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], coord.ErrInvalidVersion) {
		t.Fatalf("Errors = %v, want one ErrInvalidVersion", result.Errors)
	}
	if _, err := os.Stat(filepath.Join(dest, "rules_go", "1.0", "MODULE.bazel")); err != nil {
		t.Errorf("valid version not written: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(parent, "x"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "y" {
		t.Errorf("files written outside the destination: %v", entries)
	}
}

func TestRunner_InvalidModuleOverride(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	reg := setupHostileRegistry(t, parent)
	dest := filepath.Join(parent, "dest")

	result, err := NewService(hostileWorkflow(reg, &config.Registry{Type: "file", URL: dest})).
		Run(context.Background(), "hostile", Options{Modules: []string{"../reg/modules/rules_go", "Bad"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("Errors = %v, want 2", result.Errors)
	}
	for _, e := range result.Errors {
		if !errors.Is(e, coord.ErrInvalidModuleName) {
			t.Errorf("error %v, want ErrInvalidModuleName", e)
		}
	}
	if result.ModulesWritten != 0 {
		t.Errorf("ModulesWritten = %d, want 0", result.ModulesWritten)
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 0 {
		t.Errorf("destination not empty: %v", entries)
	}
}

func TestRunner_HostileVersionHTTPDestination(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.RequestURI)
		mu.Unlock()
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	reg := setupHostileRegistry(t, t.TempDir())
	result, err := NewService(hostileWorkflow(reg, &config.Registry{Type: "http_put", URL: server.URL + "/r"})).
		Run(context.Background(), "hostile", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], coord.ErrInvalidVersion) {
		t.Fatalf("Errors = %v, want one ErrInvalidVersion", result.Errors)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) == 0 {
		t.Fatal("no requests for the valid version")
	}
	for _, r := range requests {
		if strings.Contains(r, "..") || strings.Contains(strings.ToLower(r), "%2e") || strings.Contains(r, "ESCAPED") {
			t.Errorf("request for an invalid coordinate: %s", r)
		}
	}
}
