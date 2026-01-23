package registry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// setupTestRegistry creates a temporary BCR-compatible registry structure.
func setupTestRegistry(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	// Create modules directory
	modulesDir := filepath.Join(root, ModulesDir)
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create rules_go module
	rulesGoDir := filepath.Join(modulesDir, "rules_go")
	if err := os.MkdirAll(rulesGoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write metadata.json
	metadataJSON := `{
  "homepage": "https://github.com/bazelbuild/rules_go",
  "maintainers": [{"name": "Test User", "email": "test@example.com"}],
  "versions": ["0.49.0", "0.50.0", "0.51.0-rc1"],
  "yanked_versions": {"0.49.0": "buggy"}
}`
	if err := os.WriteFile(filepath.Join(rulesGoDir, MetadataFile), []byte(metadataJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create version directory
	versionDir := filepath.Join(rulesGoDir, "0.50.0")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write MODULE.bazel
	moduleBazel := `module(name = "rules_go", version = "0.50.0")
bazel_dep(name = "platforms", version = "0.0.9")`
	if err := os.WriteFile(filepath.Join(versionDir, ModuleBazelFile), []byte(moduleBazel), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create another module (protobuf)
	protobufDir := filepath.Join(modulesDir, "protobuf")
	if err := os.MkdirAll(protobufDir, 0o755); err != nil {
		t.Fatal(err)
	}

	protobufMetadata := `{"versions": ["3.19.0", "3.20.0"]}`
	if err := os.WriteFile(filepath.Join(protobufDir, MetadataFile), []byte(protobufMetadata), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestFileRegistry_Type(t *testing.T) {
	reg := NewFileRegistry("/some/path")
	if got := reg.Type(); got != TypeFile {
		t.Errorf("Type() = %q, want %q", got, TypeFile)
	}
}

func TestFileRegistry_String(t *testing.T) {
	reg := NewFileRegistry("/some/path")
	got := reg.String()
	if got != "file:///some/path" {
		t.Errorf("String() = %q, want %q", got, "file:///some/path")
	}
}

func TestFileRegistry_ListModules(t *testing.T) {
	root := setupTestRegistry(t)
	reg := NewFileRegistry(root)
	ctx := context.Background()

	modules, err := reg.ListModules(ctx)
	if err != nil {
		t.Fatalf("ListModules() error = %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("ListModules() returned %d modules, want 2", len(modules))
	}

	// Check both modules exist (order may vary)
	found := make(map[string]bool)
	for _, m := range modules {
		found[m] = true
	}

	if !found["rules_go"] {
		t.Error("ListModules() missing rules_go")
	}
	if !found["protobuf"] {
		t.Error("ListModules() missing protobuf")
	}
}

func TestFileRegistry_ListModules_NoModulesDir(t *testing.T) {
	root := t.TempDir() // empty directory
	reg := NewFileRegistry(root)
	ctx := context.Background()

	_, err := reg.ListModules(ctx)
	if err == nil {
		t.Error("ListModules() expected error for missing modules dir")
	}
	if !errors.Is(err, ErrListingNotSupported) {
		t.Errorf("ListModules() error = %v, want ErrListingNotSupported", err)
	}
}

func TestFileRegistry_GetMetadata(t *testing.T) {
	root := setupTestRegistry(t)
	reg := NewFileRegistry(root)
	ctx := context.Background()

	meta, err := reg.GetMetadata(ctx, "rules_go")
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if meta.Homepage != "https://github.com/bazelbuild/rules_go" {
		t.Errorf("Homepage = %q, want %q", meta.Homepage, "https://github.com/bazelbuild/rules_go")
	}

	if len(meta.Versions) != 3 {
		t.Errorf("len(Versions) = %d, want 3", len(meta.Versions))
	}

	if meta.LatestVersion() != "0.50.0" {
		t.Errorf("LatestVersion() = %q, want %q", meta.LatestVersion(), "0.50.0")
	}
}

func TestFileRegistry_GetMetadata_NotFound(t *testing.T) {
	root := setupTestRegistry(t)
	reg := NewFileRegistry(root)
	ctx := context.Background()

	_, err := reg.GetMetadata(ctx, "nonexistent")
	if err == nil {
		t.Error("GetMetadata() expected error for nonexistent module")
	}
	if !errors.Is(err, ErrModuleNotFound) {
		t.Errorf("GetMetadata() error = %v, want ErrModuleNotFound", err)
	}
}

func TestFileRegistry_GetModuleBazel(t *testing.T) {
	root := setupTestRegistry(t)
	reg := NewFileRegistry(root)
	ctx := context.Background()

	content, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	if err != nil {
		t.Fatalf("GetModuleBazel() error = %v", err)
	}

	expected := `module(name = "rules_go", version = "0.50.0")
bazel_dep(name = "platforms", version = "0.0.9")`
	if string(content) != expected {
		t.Errorf("GetModuleBazel() = %q, want %q", string(content), expected)
	}
}

func TestFileRegistry_GetModuleBazel_ModuleNotFound(t *testing.T) {
	root := setupTestRegistry(t)
	reg := NewFileRegistry(root)
	ctx := context.Background()

	_, err := reg.GetModuleBazel(ctx, "nonexistent", "1.0.0")
	if !errors.Is(err, ErrModuleNotFound) {
		t.Errorf("GetModuleBazel() error = %v, want ErrModuleNotFound", err)
	}
}

func TestFileRegistry_GetModuleBazel_VersionNotFound(t *testing.T) {
	root := setupTestRegistry(t)
	reg := NewFileRegistry(root)
	ctx := context.Background()

	_, err := reg.GetModuleBazel(ctx, "rules_go", "9.9.9")
	if !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("GetModuleBazel() error = %v, want ErrVersionNotFound", err)
	}
}
