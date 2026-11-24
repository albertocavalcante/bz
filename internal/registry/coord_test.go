package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/albertocavalcante/bz/internal/coord"
)

// TestFileRegistry_RejectsInvalidCoordinates places readable files where a
// traversal would reach them and checks that the registry refuses the
// coordinates instead of reading outside its modules directory.
func TestFileRegistry_RejectsInvalidCoordinates(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	root := filepath.Join(parent, "reg")
	writeTestFile(t, filepath.Join(root, "modules", "rules_go", "metadata.json"), `{"versions":["1.0"]}`)
	writeTestFile(t, filepath.Join(root, "modules", "rules_go", "1.0", "MODULE.bazel"), "module()")
	// Targets a traversal would resolve to.
	writeTestFile(t, filepath.Join(parent, "esc", "MODULE.bazel"), "outside")
	writeTestFile(t, filepath.Join(parent, "esc", "source.json"), "outside")
	writeTestFile(t, filepath.Join(parent, "metadata.json"), `{"versions":["9"]}`)

	reg := NewFileRegistry(root)
	ctx := context.Background()

	if _, err := reg.GetMetadata(ctx, "../.."); !errors.Is(err, coord.ErrInvalidModuleName) {
		t.Errorf("GetMetadata(../..) = %v, want ErrInvalidModuleName", err)
	}
	for _, bad := range []string{"../../../esc", "..", ".", ""} {
		if _, err := reg.GetModuleBazel(ctx, "rules_go", bad); !errors.Is(err, coord.ErrInvalidVersion) {
			t.Errorf("GetModuleBazel(rules_go, %q) = %v, want ErrInvalidVersion", bad, err)
		}
		if _, err := reg.GetSource(ctx, "rules_go", bad); !errors.Is(err, coord.ErrInvalidVersion) {
			t.Errorf("GetSource(rules_go, %q) = %v, want ErrInvalidVersion", bad, err)
		}
	}
	if _, err := reg.GetModuleBazel(ctx, "../x", "1.0"); !errors.Is(err, coord.ErrInvalidModuleName) {
		t.Errorf("GetModuleBazel(../x) = %v, want ErrInvalidModuleName", err)
	}

	// Valid coordinates still work.
	if _, err := reg.GetModuleBazel(ctx, "rules_go", "1.0"); err != nil {
		t.Errorf("GetModuleBazel(rules_go, 1.0) = %v", err)
	}
}

// TestHTTPRegistry_RejectsInvalidCoordinates checks that invalid coordinates
// fail before any request is sent.
func TestHTTPRegistry_RejectsInvalidCoordinates(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path)
		mu.Unlock()
		http.NotFound(w, r)
	}))
	defer server.Close()

	reg := NewHTTPRegistry(server.URL)
	ctx := context.Background()

	if _, err := reg.GetMetadata(ctx, "../etc"); !errors.Is(err, coord.ErrInvalidModuleName) {
		t.Errorf("GetMetadata = %v, want ErrInvalidModuleName", err)
	}
	if _, err := reg.GetModuleBazel(ctx, "rules_go", "../../x"); !errors.Is(err, coord.ErrInvalidVersion) {
		t.Errorf("GetModuleBazel = %v, want ErrInvalidVersion", err)
	}
	if _, err := reg.GetSource(ctx, "rules_go", ".."); !errors.Is(err, coord.ErrInvalidVersion) {
		t.Errorf("GetSource = %v, want ErrInvalidVersion", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 0 {
		t.Errorf("sent %d request(s) for invalid coordinates: %v", len(requests), requests)
	}
}

func TestCheckModulesRel(t *testing.T) {
	t.Parallel()

	if err := checkModulesRel(ModuleBazelPath("rules_go", "1.0")); err != nil {
		t.Errorf("valid path rejected: %v", err)
	}
	// The path helpers clean their output, so a traversal surfaces as a path
	// outside modules/ rather than as a ".." segment.
	for _, rel := range []string{
		MetadataPath(".."),
		ModuleBazelPath("rules_go", "../../.."),
		"modules/../x",
		"modules//x",
		"other/x",
	} {
		if err := checkModulesRel(rel); !errors.Is(err, coord.ErrEscapesBase) {
			t.Errorf("checkModulesRel(%q) = %v, want ErrEscapesBase", rel, err)
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
