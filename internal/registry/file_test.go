package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestFileRegistry_Type(t *testing.T) {
	t.Parallel()
	reg := NewFileRegistry("/some/path")
	if got := reg.Type(); got != TypeFile {
		t.Errorf("Type() = %q, want %q", got, TypeFile)
	}
}

func TestFileRegistry_String(t *testing.T) {
	t.Parallel()
	reg := NewFileRegistry("/some/path")
	got := reg.String()
	if got != "file:///some/path" {
		t.Errorf("String() = %q, want %q", got, "file:///some/path")
	}
}

func TestFileRegistry_ListModules(t *testing.T) {
	t.Parallel()
	root := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions:       []string{"0.49.0", "0.50.0", "0.51.0-rc1"},
			YankedVersions: map[string]string{"0.49.0": "buggy"},
		},
		"protobuf": {Versions: []string{"3.19.0", "3.20.0"}},
	})
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
	t.Parallel()
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
	t.Parallel()
	root := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions:       []string{"0.49.0", "0.50.0", "0.51.0-rc1"},
			YankedVersions: map[string]string{"0.49.0": "buggy"},
		},
		"protobuf": {Versions: []string{"3.19.0", "3.20.0"}},
	})
	reg := NewFileRegistry(root)
	ctx := context.Background()

	meta, err := reg.GetMetadata(ctx, "rules_go")
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if len(meta.Versions) != 3 {
		t.Errorf("len(Versions) = %d, want 3", len(meta.Versions))
	}

	// Latest stable version should be 0.50.0 (0.51.0-rc1 is prerelease, 0.49.0 is yanked)
	if meta.LatestVersion() != "0.50.0" {
		t.Errorf("LatestVersion() = %q, want %q", meta.LatestVersion(), "0.50.0")
	}
}

func TestFileRegistry_GetMetadata_NotFound(t *testing.T) {
	t.Parallel()
	root := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.0"}},
	})
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
	t.Parallel()
	root := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.0"},
			Deps:     map[string][]string{"0.50.0": {"platforms@0.0.9"}},
		},
		"platforms": {Versions: []string{"0.0.9"}},
	})
	reg := NewFileRegistry(root)
	ctx := context.Background()

	content, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	if err != nil {
		t.Fatalf("GetModuleBazel() error = %v", err)
	}

	// Check that the module content looks correct
	if len(content) == 0 {
		t.Error("GetModuleBazel() returned empty content")
	}
	if string(content)[:len("module(")] != "module(" {
		t.Errorf("GetModuleBazel() content doesn't start with 'module(': %s", string(content)[:50])
	}
}

func TestFileRegistry_GetModuleBazel_ModuleNotFound(t *testing.T) {
	t.Parallel()
	root := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.0"}},
	})
	reg := NewFileRegistry(root)
	ctx := context.Background()

	_, err := reg.GetModuleBazel(ctx, "nonexistent", "1.0.0")
	if !errors.Is(err, ErrModuleNotFound) {
		t.Errorf("GetModuleBazel() error = %v, want ErrModuleNotFound", err)
	}
}

func TestFileRegistry_GetModuleBazel_VersionNotFound(t *testing.T) {
	t.Parallel()
	root := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.0"}},
	})
	reg := NewFileRegistry(root)
	ctx := context.Background()

	_, err := reg.GetModuleBazel(ctx, "rules_go", "9.9.9")
	if !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("GetModuleBazel() error = %v, want ErrVersionNotFound", err)
	}
}
