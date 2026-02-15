package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/albertocavalcante/go-bcr"
)

// mockBCRRegistry implements bcr.Registry for testing.
type mockBCRRegistry struct {
	metadata    *bcr.Metadata
	moduleFile  []byte
	source      *bcr.Source
	modules     []string
	metaErr     error
	moduleErr   error
	sourceErr   error
	listErr     error
	baseURL     string
	registryTyp string
}

func (m *mockBCRRegistry) Metadata(ctx context.Context, module string) (*bcr.Metadata, error) {
	if m.metaErr != nil {
		return nil, m.metaErr
	}
	return m.metadata, nil
}

func (m *mockBCRRegistry) ModuleFile(ctx context.Context, module, version string) ([]byte, error) {
	if m.moduleErr != nil {
		return nil, m.moduleErr
	}
	return m.moduleFile, nil
}

func (m *mockBCRRegistry) Source(ctx context.Context, module, version string) (*bcr.Source, error) {
	if m.sourceErr != nil {
		return nil, m.sourceErr
	}
	return m.source, nil
}

func (m *mockBCRRegistry) ListModules(ctx context.Context) ([]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.modules, nil
}

func (m *mockBCRRegistry) String() string {
	return m.baseURL
}

func (m *mockBCRRegistry) Type() string {
	return m.registryTyp
}

func TestBCRAdapter_GetMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock := &mockBCRRegistry{
			metadata: &bcr.Metadata{
				Versions:       []string{"1.0.0", "2.0.0"},
				YankedVersions: map[string]string{"1.0.0": "broken"},
				Homepage:       "https://example.com",
				Maintainers: []bcr.Maintainer{
					{Name: "Alice", Email: "alice@example.com", GitHub: "alice"},
				},
				Repository: []string{"github:example/repo"},
			},
			baseURL:     "https://bcr.bazel.build",
			registryTyp: "https",
		}

		adapter := FromBCR(mock)
		meta, err := adapter.GetMetadata(ctx, "testmod")
		if err != nil {
			t.Fatalf("GetMetadata() error = %v", err)
		}

		if len(meta.Versions) != 2 {
			t.Errorf("Versions = %v, want 2 elements", meta.Versions)
		}
		if meta.Homepage != "https://example.com" {
			t.Errorf("Homepage = %q, want %q", meta.Homepage, "https://example.com")
		}
		if len(meta.Maintainers) != 1 {
			t.Errorf("Maintainers = %v, want 1 element", meta.Maintainers)
		}
		if meta.Maintainers[0].Name != "Alice" {
			t.Errorf("Maintainer.Name = %q, want %q", meta.Maintainers[0].Name, "Alice")
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		mock := &mockBCRRegistry{
			metaErr:     &bcr.NotFoundError{Module: "nonexistent"},
			baseURL:     "https://bcr.bazel.build",
			registryTyp: "https",
		}

		adapter := FromBCR(mock)
		_, err := adapter.GetMetadata(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrModuleNotFound) {
			t.Errorf("error = %v, want ErrModuleNotFound", err)
		}
	})
}

func TestBCRAdapter_GetModuleBazel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		content := []byte(`module(name = "testmod", version = "1.0.0")`)
		mock := &mockBCRRegistry{
			moduleFile:  content,
			baseURL:     "https://bcr.bazel.build",
			registryTyp: "https",
		}

		adapter := FromBCR(mock)
		got, err := adapter.GetModuleBazel(ctx, "testmod", "1.0.0")
		if err != nil {
			t.Fatalf("GetModuleBazel() error = %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("content = %q, want %q", got, content)
		}
	})

	t.Run("version not found", func(t *testing.T) {
		t.Parallel()
		mock := &mockBCRRegistry{
			moduleErr:   &bcr.NotFoundError{Module: "testmod", Version: "9.9.9"},
			baseURL:     "https://bcr.bazel.build",
			registryTyp: "https",
		}

		adapter := FromBCR(mock)
		_, err := adapter.GetModuleBazel(ctx, "testmod", "9.9.9")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrVersionNotFound) {
			t.Errorf("error = %v, want ErrVersionNotFound", err)
		}
	})
}

func TestBCRAdapter_ListModules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock := &mockBCRRegistry{
			modules:     []string{"rules_go", "rules_python"},
			baseURL:     "file:///path/to/registry",
			registryTyp: "file",
		}

		adapter := FromBCR(mock)
		got, err := adapter.ListModules(ctx)
		if err != nil {
			t.Fatalf("ListModules() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("got %d modules, want 2", len(got))
		}
	})

	t.Run("not supported", func(t *testing.T) {
		t.Parallel()
		mock := &mockBCRRegistry{
			listErr:     bcr.ErrListingNotSupported,
			baseURL:     "https://bcr.bazel.build",
			registryTyp: "https",
		}

		adapter := FromBCR(mock)
		_, err := adapter.ListModules(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrListingNotSupported) {
			t.Errorf("error = %v, want ErrListingNotSupported", err)
		}
	})
}

func TestBCRAdapter_Type(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		typ  string
	}{
		{"https", "https"},
		{"http", "http"},
		{"file", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mock := &mockBCRRegistry{
				registryTyp: tt.typ,
			}
			adapter := FromBCR(mock)
			if got := adapter.Type(); got != tt.typ {
				t.Errorf("Type() = %q, want %q", got, tt.typ)
			}
		})
	}
}

func TestBCRAdapter_String(t *testing.T) {
	t.Parallel()
	mock := &mockBCRRegistry{
		baseURL: "https://bcr.bazel.build",
	}
	adapter := FromBCR(mock)
	if got := adapter.String(); got != "https://bcr.bazel.build" {
		t.Errorf("String() = %q, want %q", got, "https://bcr.bazel.build")
	}
}
