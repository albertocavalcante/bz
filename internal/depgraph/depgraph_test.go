package depgraph

import (
	"context"
	"testing"

	"github.com/albertocavalcante/bz/internal/registry"
	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestFetchDeps(t *testing.T) {
	t.Parallel()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps: map[string][]string{
				"0.50.1": {"platforms@0.0.10"},
			},
		},
		"platforms": {Versions: []string{"0.0.10"}},
	})

	reg := registry.NewFileRegistry(registryDir)
	deps, err := FetchDeps(context.Background(), reg, ModuleRef{Name: "rules_go", Version: "0.50.1"})
	if err != nil {
		t.Fatalf("FetchDeps returned error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("FetchDeps returned %d deps, want 1", len(deps))
	}
	if deps[0].Name != "platforms" || deps[0].Version != "0.0.10" {
		t.Fatalf("FetchDeps returned unexpected dep: %+v", deps[0])
	}
}

func TestResolveTransitive(t *testing.T) {
	t.Parallel()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"a": {
			Versions: []string{"1.0.0"},
			Deps: map[string][]string{
				"1.0.0": {"b@1.0.0", "c@1.0.0"},
			},
		},
		"b": {
			Versions: []string{"1.0.0"},
			Deps: map[string][]string{
				"1.0.0": {"d@1.0.0"},
			},
		},
		"c": {
			Versions: []string{"1.0.0"},
			Deps: map[string][]string{
				"1.0.0": {"d@1.0.0"},
			},
		},
		"d": {Versions: []string{"1.0.0"}},
	})

	reg := registry.NewFileRegistry(registryDir)
	resolved := ResolveTransitive(context.Background(), reg, []ModuleRef{
		{Name: "a", Version: "1.0.0"},
	})

	seen := make(map[string]bool)
	for _, ref := range resolved {
		seen[ref.Key()] = true
	}

	for _, key := range []string{"a@1.0.0", "b@1.0.0", "c@1.0.0", "d@1.0.0"} {
		if !seen[key] {
			t.Fatalf("ResolveTransitive missing %s", key)
		}
	}
}
