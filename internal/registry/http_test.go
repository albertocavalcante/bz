package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPRegistry_Type(t *testing.T) {
	t.Parallel()
	reg := NewHTTPRegistry("https://bcr.bazel.build")
	if got := reg.Type(); got != TypeHTTPS {
		t.Errorf("Type() = %q, want %q", got, TypeHTTPS)
	}

	reg2 := NewHTTPRegistry("http://example.com")
	if got := reg2.Type(); got != TypeHTTP {
		t.Errorf("Type() = %q, want %q", got, TypeHTTP)
	}
}

func TestHTTPRegistry_String(t *testing.T) {
	t.Parallel()
	reg := NewHTTPRegistry("https://bcr.bazel.build")
	if got := reg.String(); got != "https://bcr.bazel.build" {
		t.Errorf("String() = %q, want %q", got, "https://bcr.bazel.build")
	}
}

func TestHTTPRegistry_GetMetadata(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/rules_go/metadata.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"homepage": "https://github.com/bazelbuild/rules_go",
				"versions": ["0.49.0", "0.50.0"],
				"yanked_versions": {}
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	reg := NewHTTPRegistry(server.URL)
	ctx := context.Background()

	meta, err := reg.GetMetadata(ctx, "rules_go")
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if meta.Homepage != "https://github.com/bazelbuild/rules_go" {
		t.Errorf("Homepage = %q", meta.Homepage)
	}

	if len(meta.Versions) != 2 {
		t.Errorf("len(Versions) = %d, want 2", len(meta.Versions))
	}
}

func TestHTTPRegistry_GetMetadata_NotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	reg := NewHTTPRegistry(server.URL)
	ctx := context.Background()

	_, err := reg.GetMetadata(ctx, "nonexistent")
	if !errors.Is(err, ErrModuleNotFound) {
		t.Errorf("GetMetadata() error = %v, want ErrModuleNotFound", err)
	}
}

func TestHTTPRegistry_GetModuleBazel(t *testing.T) {
	t.Parallel()
	expectedContent := `module(name = "rules_go", version = "0.50.0")`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/modules/rules_go/0.50.0/MODULE.bazel":
			w.Write([]byte(expectedContent))
		case "/modules/rules_go/metadata.json":
			// Module exists
			w.Write([]byte(`{"versions": ["0.50.0"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reg := NewHTTPRegistry(server.URL)
	ctx := context.Background()

	content, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	if err != nil {
		t.Fatalf("GetModuleBazel() error = %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("GetModuleBazel() = %q, want %q", string(content), expectedContent)
	}
}

func TestHTTPRegistry_GetModuleBazel_VersionNotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/modules/rules_go/metadata.json":
			w.Write([]byte(`{"versions": ["0.50.0"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reg := NewHTTPRegistry(server.URL)
	ctx := context.Background()

	_, err := reg.GetModuleBazel(ctx, "rules_go", "9.9.9")
	if !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("GetModuleBazel() error = %v, want ErrVersionNotFound", err)
	}
}

func TestHTTPRegistry_ListModules_WithIndexJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/index.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`["protobuf", "rules_go", "rules_python"]`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	reg := NewHTTPRegistry(server.URL)
	ctx := context.Background()

	modules, err := reg.ListModules(ctx)
	if err != nil {
		t.Fatalf("ListModules() error = %v", err)
	}

	if len(modules) != 3 {
		t.Errorf("ListModules() returned %d modules, want 3", len(modules))
	}

	// Should be sorted
	expected := []string{"protobuf", "rules_go", "rules_python"}
	for i, m := range modules {
		if m != expected[i] {
			t.Errorf("modules[%d] = %q, want %q", i, m, expected[i])
		}
	}
}

func TestHTTPRegistry_ListModules_WithHTMLListing(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/modules/index.json":
			http.NotFound(w, r)
		case "/modules/", "/modules":
			// Apache-style directory listing
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Index of /modules</title></head>
<body>
<h1>Index of /modules</h1>
<pre><a href="../">../</a>
<a href="protobuf/">protobuf/</a>
<a href="rules_go/">rules_go/</a>
<a href="rules_python/">rules_python/</a>
</pre>
</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reg := NewHTTPRegistry(server.URL)
	ctx := context.Background()

	modules, err := reg.ListModules(ctx)
	if err != nil {
		t.Fatalf("ListModules() error = %v", err)
	}

	if len(modules) != 3 {
		t.Errorf("ListModules() returned %d modules, want 3", len(modules))
	}
}

func TestHTTPRegistry_ListModules_NotSupported(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No index.json, no directory listing
		http.NotFound(w, r)
	}))
	defer server.Close()

	reg := NewHTTPRegistry(server.URL)
	ctx := context.Background()

	_, err := reg.ListModules(ctx)
	if !errors.Is(err, ErrListingNotSupported) {
		t.Errorf("ListModules() error = %v, want ErrListingNotSupported", err)
	}
}

func TestHTTPRegistry_ListModules_NginxAutoindex(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/modules/index.json":
			http.NotFound(w, r)
		case "/modules/", "/modules":
			// Nginx-style JSON autoindex
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[
				{"name": "protobuf", "type": "directory"},
				{"name": "rules_go", "type": "directory"},
				{"name": "index.json", "type": "file"}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reg := NewHTTPRegistry(server.URL)
	ctx := context.Background()

	modules, err := reg.ListModules(ctx)
	if err != nil {
		t.Fatalf("ListModules() error = %v", err)
	}

	// Should only include directories, not files
	if len(modules) != 2 {
		t.Errorf("ListModules() returned %d modules, want 2", len(modules))
	}
}
