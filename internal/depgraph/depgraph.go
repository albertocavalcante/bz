// Package depgraph provides shared dependency graph traversal helpers.
package depgraph

import (
	"context"

	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
)

// ModuleRef identifies a concrete module version.
type ModuleRef struct {
	Name    string
	Version string
}

// Key returns a stable identity key in name@version format.
func (m ModuleRef) Key() string {
	return m.Name + "@" + m.Version
}

// FromModuleFileDeps converts direct deps from a parsed MODULE.bazel file.
func FromModuleFileDeps(f *module.File) []ModuleRef {
	refs := make([]ModuleRef, 0, len(f.Deps))
	for _, dep := range f.Deps {
		refs = append(refs, ModuleRef{
			Name:    dep.Name.String(),
			Version: dep.Version.String(),
		})
	}
	return refs
}

// FetchDeps fetches and parses a module's transitive dependencies.
func FetchDeps(ctx context.Context, reg registry.Registry, ref ModuleRef) ([]ModuleRef, error) {
	content, err := reg.GetModuleBazel(ctx, ref.Name, ref.Version)
	if err != nil {
		return nil, err
	}

	modFile, err := module.LoadContent(ref.Name, content)
	if err != nil {
		return nil, err
	}

	return FromModuleFileDeps(modFile), nil
}

// ResolveTransitive resolves all transitive dependencies breadth-first.
// Missing modules or parse errors are skipped so traversal can continue.
func ResolveTransitive(ctx context.Context, reg registry.Registry, initial []ModuleRef) []ModuleRef {
	seen := make(map[string]bool)
	result := make([]ModuleRef, 0, len(initial))
	queue := append([]ModuleRef(nil), initial...)

	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]

		if seen[ref.Key()] {
			continue
		}
		seen[ref.Key()] = true
		result = append(result, ref)

		deps, err := FetchDeps(ctx, reg, ref)
		if err != nil {
			continue
		}

		for _, dep := range deps {
			if !seen[dep.Key()] {
				queue = append(queue, dep)
			}
		}
	}

	return result
}
