package mod

import (
	"fmt"

	"github.com/albertocavalcante/bz/internal/module"
)

// includedDeps returns the dependencies that live in files pulled in through
// include(), keyed by module name. The editing commands only rewrite the root
// MODULE.bazel, so they use it to refuse work that would silently miss, or
// duplicate, a declaration in an included file. It is empty when the root file
// has no include() calls.
func includedDeps(root *module.File, modulePath string) (map[string]module.Origin, error) {
	if len(root.Includes) == 0 {
		return map[string]module.Origin{}, nil
	}
	full, err := module.Load(modulePath)
	if err != nil {
		return nil, err
	}
	deps := make(map[string]module.Origin)
	for _, dep := range full.Deps {
		if o := full.OriginOf(dep); !o.IsRoot() {
			deps[dep.Name.String()] = o
		}
	}
	return deps, nil
}

func includedEditError(name string, o module.Origin) error {
	return fmt.Errorf("%s is declared in %s:%d; editing included files is not supported yet", name, o.File, o.Pos.Start.Line)
}
