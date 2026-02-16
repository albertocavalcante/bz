package tui

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
)

type failingSearchRegistry struct {
	err error
}

func (f *failingSearchRegistry) GetMetadata(_ context.Context, _ string) (*registry.Metadata, error) {
	return nil, f.err
}

func (f *failingSearchRegistry) GetModuleBazel(_ context.Context, _, _ string) ([]byte, error) {
	return nil, f.err
}

func (f *failingSearchRegistry) ListModules(_ context.Context) ([]string, error) {
	return nil, f.err
}

func (f *failingSearchRegistry) Type() string {
	return "failing"
}

func (f *failingSearchRegistry) String() string {
	return "failing://test"
}

func TestSearchModules(t *testing.T) {
	t.Run("returns results with latest version", func(t *testing.T) {
		reg := registry.NewMockRegistry(map[string]*registry.Metadata{
			"rules_go": {
				Versions: []string{"0.50.0", "0.51.0"},
			},
			"protobuf": {
				Versions: []string{"27.0.0"},
			},
		})

		got, err := searchModules(context.Background(), reg, "rules")
		if err != nil {
			t.Fatalf("searchModules() error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len(searchModules()) = %d, want 1", len(got))
		}
		if got[0].Name != "rules_go" {
			t.Fatalf("result.Name = %q, want %q", got[0].Name, "rules_go")
		}
		if got[0].Version != "0.51.0" {
			t.Fatalf("result.Version = %q, want %q", got[0].Version, "0.51.0")
		}
	})

	t.Run("empty query returns empty results", func(t *testing.T) {
		reg := registry.NewMockRegistry(map[string]*registry.Metadata{
			"rules_go": {Versions: []string{"0.51.0"}},
		})

		got, err := searchModules(context.Background(), reg, "   ")
		if err != nil {
			t.Fatalf("searchModules() error = %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("len(searchModules()) = %d, want 0", len(got))
		}
	})

	t.Run("registry list failure returns error", func(t *testing.T) {
		wantErr := errors.New("list failed")
		reg := &failingSearchRegistry{err: wantErr}

		_, err := searchModules(context.Background(), reg, "rules")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
	})
}

func TestListLocalDeps(t *testing.T) {
	t.Cleanup(func() {
		findAndLoadModule = module.FindAndLoad
	})

	t.Run("missing module file returns empty deps", func(t *testing.T) {
		findAndLoadModule = func() (*module.File, error) {
			return nil, fmt.Errorf("wrapped: %w", module.ErrModuleFileNotFound)
		}

		msg := ListLocalDeps()()
		depsMsg, ok := msg.(DepsListedMsg)
		if !ok {
			t.Fatalf("ListLocalDeps()() = %T, want DepsListedMsg", msg)
		}
		if depsMsg.File == nil {
			t.Fatal("DepsListedMsg.File is nil")
		}
		if len(depsMsg.File.Deps) != 0 {
			t.Fatalf("len(DepsListedMsg.File.Deps) = %d, want 0", len(depsMsg.File.Deps))
		}
	})

	t.Run("non-not-found error returns ErrMsg", func(t *testing.T) {
		wantErr := errors.New("load failed")
		findAndLoadModule = func() (*module.File, error) {
			return nil, wantErr
		}

		msg := ListLocalDeps()()
		errMsg, ok := msg.(ErrMsg)
		if !ok {
			t.Fatalf("ListLocalDeps()() = %T, want ErrMsg", msg)
		}
		if !errors.Is(errMsg.Err, wantErr) {
			t.Fatalf("ErrMsg.Err = %v, want wrapped %v", errMsg.Err, wantErr)
		}
	})

	t.Run("success returns deps listed message", func(t *testing.T) {
		wantFile := &module.File{}
		findAndLoadModule = func() (*module.File, error) {
			return wantFile, nil
		}

		msg := ListLocalDeps()()
		depsMsg, ok := msg.(DepsListedMsg)
		if !ok {
			t.Fatalf("ListLocalDeps()() = %T, want DepsListedMsg", msg)
		}
		if depsMsg.File != wantFile {
			t.Fatalf("DepsListedMsg.File = %p, want %p", depsMsg.File, wantFile)
		}
	})
}
