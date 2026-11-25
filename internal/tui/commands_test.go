package tui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
	"github.com/albertocavalcante/bz/internal/testutil"
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

func TestSearchModulesOffline(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	cli.ResetCachedConfig()
	t.Cleanup(cli.ResetCachedConfig)
	t.Setenv("BZ_OFFLINE", "1")
	t.Setenv("BZ_REGISTRY", server.URL)
	msg := SearchModules("fixture")()
	errMsg, ok := msg.(ErrMsg)
	if !ok {
		t.Fatalf("offline search got %T, want ErrMsg", msg)
	}
	var offlineErr *cli.OfflineModeError
	if !errors.As(errMsg.Err, &offlineErr) || requests.Load() != 0 {
		t.Errorf("offline search made %d requests and returned %v", requests.Load(), errMsg.Err)
	}

	t.Setenv("BZ_REGISTRY", testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"fixture": {Versions: []string{"1.0.0"}},
	}))
	msg = SearchModules("fixture")()
	result, ok := msg.(SearchResultsMsg)
	if !ok || len(result.Results) != 1 || result.Results[0].Version != "1.0.0" {
		t.Errorf("offline search of local registry got %#v", msg)
	}
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
		if !depsMsg.MissingModule {
			t.Fatal("DepsListedMsg.MissingModule = false, want true")
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

func TestInitModule(t *testing.T) {
	origGetWorkingDir := getWorkingDir
	origInitModuleInDir := initModuleInDir
	t.Cleanup(func() {
		getWorkingDir = origGetWorkingDir
		initModuleInDir = origInitModuleInDir
	})

	t.Run("working directory error returns ErrMsg", func(t *testing.T) {
		wantErr := errors.New("cwd failed")
		getWorkingDir = func() (string, error) {
			return "", wantErr
		}
		initModuleInDir = func(_, _, _ string, _ bool) (string, string, error) {
			t.Fatal("initModuleInDir should not be called")
			return "", "", nil
		}

		msg := InitModule("", "", false)()
		errMsg, ok := msg.(ErrMsg)
		if !ok {
			t.Fatalf("InitModule()() = %T, want ErrMsg", msg)
		}
		if !errors.Is(errMsg.Err, wantErr) {
			t.Fatalf("ErrMsg.Err = %v, want wrapped %v", errMsg.Err, wantErr)
		}
	})

	t.Run("init error returns ErrMsg", func(t *testing.T) {
		wantErr := errors.New("init failed")
		getWorkingDir = func() (string, error) {
			return "/tmp/work", nil
		}
		initModuleInDir = func(_, _, _ string, _ bool) (string, string, error) {
			return "", "", wantErr
		}

		msg := InitModule("", "", false)()
		errMsg, ok := msg.(ErrMsg)
		if !ok {
			t.Fatalf("InitModule()() = %T, want ErrMsg", msg)
		}
		if !errors.Is(errMsg.Err, wantErr) {
			t.Fatalf("ErrMsg.Err = %v, want wrapped %v", errMsg.Err, wantErr)
		}
	})

	t.Run("derived invalid name gets TUI help", func(t *testing.T) {
		getWorkingDir = func() (string, error) {
			return "/tmp/work", nil
		}
		initModuleInDir = func(_, _, _ string, _ bool) (string, string, error) {
			return "", "", fmt.Errorf("%w \"proj-\" (%w)", coord.ErrInvalidModuleName, module.ErrNameFromDir)
		}

		errMsg, ok := InitModule("", "", false)().(ErrMsg)
		if !ok {
			t.Fatal("InitModule()() is not ErrMsg")
		}
		want := "invalid module name \"proj-\" (derived from the directory name)\nhelp: rename the directory, or run `bz init --name <name>`"
		if errMsg.Err.Error() != want {
			t.Fatalf("ErrMsg.Err = %q, want %q", errMsg.Err, want)
		}
	})

	t.Run("success returns ModuleInitializedMsg", func(t *testing.T) {
		getWorkingDir = func() (string, error) {
			return "/tmp/work", nil
		}
		initModuleInDir = func(dir, name, version string, force bool) (string, string, error) {
			if dir != "/tmp/work" {
				t.Fatalf("dir = %q, want %q", dir, "/tmp/work")
			}
			if name != "demo_mod" {
				t.Fatalf("name = %q, want %q", name, "demo_mod")
			}
			if version != "" {
				t.Fatalf("version = %q, want empty", version)
			}
			if force {
				t.Fatal("force = true, want false")
			}
			return "/tmp/work/MODULE.bazel", "demo_mod", nil
		}

		msg := InitModule("demo_mod", "", false)()
		initMsg, ok := msg.(ModuleInitializedMsg)
		if !ok {
			t.Fatalf("InitModule()() = %T, want ModuleInitializedMsg", msg)
		}
		if initMsg.Name != "demo_mod" {
			t.Fatalf("ModuleInitializedMsg.Name = %q, want %q", initMsg.Name, "demo_mod")
		}
		if initMsg.Version != module.DefaultModuleVersion {
			t.Fatalf("ModuleInitializedMsg.Version = %q, want %q", initMsg.Version, module.DefaultModuleVersion)
		}
		if initMsg.Path != "/tmp/work/MODULE.bazel" {
			t.Fatalf("ModuleInitializedMsg.Path = %q, want %q", initMsg.Path, "/tmp/work/MODULE.bazel")
		}
	})
}

func TestAddDependency_RefusesModuleDeclaredInIncludedFile(t *testing.T) {
	dir := t.TempDir()
	root := "module(name = \"m\")\ninclude(\"//d:x.MODULE.bazel\")\n"
	if err := os.MkdirAll(filepath.Join(dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MODULE.bazel"), []byte(root), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "d", "x.MODULE.bazel"), []byte("bazel_dep(name = \"b\", version = \"1\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	errMsg, ok := AddDependency("b", "2", false)().(ErrMsg)
	if !ok || !strings.Contains(errMsg.Err.Error(), "b is already declared in d/x.MODULE.bazel:1") {
		t.Fatalf("want already-declared ErrMsg, got %#v", errMsg)
	}

	if _, ok := AddDependency("c", "3", false)().(DepAddedMsg); !ok {
		t.Fatal("adding a new module must succeed")
	}
	got, err := os.ReadFile(filepath.Join(dir, "MODULE.bazel"))
	if err != nil {
		t.Fatal(err)
	}
	if want := root + "bazel_dep(name = \"c\", version = \"3\")\n"; string(got) != want {
		t.Errorf("MODULE.bazel = %q, want %q", got, want)
	}
}
