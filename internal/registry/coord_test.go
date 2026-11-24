package registry

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

// TestFileRegistry_SymlinksCannotEscape plants symlinks inside a registry
// (as a cloned hostile registry could) and checks that reads which would
// leave the modules directory fail, while internal links keep working.
func TestFileRegistry_SymlinksCannotEscape(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	root := filepath.Join(parent, "reg")
	modules := filepath.Join(root, "modules")
	secret := "SECRET-CONTENT"
	writeTestFile(t, filepath.Join(parent, "outside", "secret"), secret)
	writeTestFile(t, filepath.Join(parent, "outside", "mod", "metadata.json"), `{"versions":["1.0"]}`)
	writeTestFile(t, filepath.Join(parent, "outside", "mod", "1.0", "MODULE.bazel"), secret)
	writeTestFile(t, filepath.Join(modules, "rules_go", "metadata.json"), `{"versions":["1.0"]}`)
	writeTestFile(t, filepath.Join(modules, "rules_go", "1.0", "MODULE.bazel"), "module()")
	writeTestFile(t, filepath.Join(modules, "evil", "metadata.json"), `{"versions":["1.0"]}`)
	if err := os.MkdirAll(filepath.Join(modules, "evil", "1.0"), 0o755); err != nil {
		t.Fatal(err)
	}

	links := map[string]string{
		// File symlink to a file outside the registry.
		filepath.Join(modules, "evil", "1.0", "MODULE.bazel"): filepath.Join(parent, "outside", "secret"),
		filepath.Join(modules, "evil", "1.0", "source.json"):  "../../../../outside/secret",
		// Directory symlink to a directory outside the registry.
		filepath.Join(modules, "evildir"): filepath.Join(parent, "outside", "mod"),
		// Internal links that stay inside modules/.
		filepath.Join(modules, "alias"):                       "rules_go",
		filepath.Join(modules, "rules_go", "1.0", "link.txt"): "MODULE.bazel",
	}
	for link, target := range links {
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	reg := NewFileRegistry(root)
	ctx := context.Background()

	check := func(what string, data []byte, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: read through an escaping symlink succeeded", what)
		}
		if strings.Contains(string(data), secret) {
			t.Errorf("%s: returned content from outside the registry", what)
		}
	}
	data, err := reg.GetModuleBazel(ctx, "evil", "1.0")
	check("file symlink MODULE.bazel", data, err)
	data, err = reg.GetSource(ctx, "evil", "1.0")
	check("relative file symlink source.json", data, err)
	data, err = reg.GetModuleBazel(ctx, "evildir", "1.0")
	check("directory symlink MODULE.bazel", data, err)
	if _, err := reg.GetMetadata(ctx, "evildir"); err == nil {
		t.Error("directory symlink metadata.json: read succeeded")
	}

	// Internal symlinks still resolve.
	if data, err := reg.GetModuleBazel(ctx, "alias", "1.0"); err != nil || string(data) != "module()" {
		t.Errorf("internal directory symlink: data=%q err=%v", data, err)
	}
	if _, err := reg.GetMetadata(ctx, "alias"); err != nil {
		t.Errorf("internal directory symlink metadata: %v", err)
	}
}

// TestFileRegistry_ModulesDirSymlink covers the base of the confinement: a
// "modules" entry that is a symlink leaving the registry is refused, while a
// "modules" link to a directory inside the registry still works.
func TestFileRegistry_ModulesDirSymlink(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	secret := "OUTSIDE-PRIVATE"
	writeTestFile(t, filepath.Join(parent, "outside", "victim", "metadata.json"), `{"versions":["1.0"]}`)
	writeTestFile(t, filepath.Join(parent, "outside", "victim", "1.0", "MODULE.bazel"), secret)

	escaping := filepath.Join(parent, "escaping")
	if err := os.MkdirAll(escaping, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(parent, "outside"), filepath.Join(escaping, "modules")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	reg := NewFileRegistry(escaping)
	ctx := context.Background()
	if data, err := reg.GetModuleBazel(ctx, "victim", "1.0"); err == nil || strings.Contains(string(data), secret) {
		t.Errorf("read through an escaping modules link: data=%q err=%v", data, err)
	}
	if _, err := reg.GetMetadata(ctx, "victim"); err == nil {
		t.Error("metadata read through an escaping modules link succeeded")
	}

	internal := filepath.Join(parent, "internal")
	writeTestFile(t, filepath.Join(internal, "data", "rules_go", "metadata.json"), `{"versions":["1.0"]}`)
	writeTestFile(t, filepath.Join(internal, "data", "rules_go", "1.0", "MODULE.bazel"), "module()")
	if err := os.Symlink("data", filepath.Join(internal, "modules")); err != nil {
		t.Fatal(err)
	}
	reg = NewFileRegistry(internal)
	if data, err := reg.GetModuleBazel(ctx, "rules_go", "1.0"); err != nil || string(data) != "module()" {
		t.Errorf("internal modules link: data=%q err=%v", data, err)
	}
}
