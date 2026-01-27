package modules

import (
	"fmt"

	"go.starlark.net/starlark"
)

// All returns all bz Starlark modules.
// Each module is a StringDict mapping function names to builtins.
func All() map[string]starlark.StringDict {
	return map[string]starlark.StringDict{
		"registry": RegistryModule(),
		"auth":     AuthModule(),
		"sync":     SyncModule(),
		"config":   ConfigModule(),
	}
}

// Predeclared returns a StringDict with all modules as predeclared values.
// This is suitable for use with starlark.ExecFile's predeclared argument.
func Predeclared() starlark.StringDict {
	result := make(starlark.StringDict)
	for name, module := range All() {
		// Wrap each module as a struct-like value with attributes
		result[name] = &moduleValue{
			name:  name,
			attrs: module,
		}
	}
	return result
}

// moduleValue wraps a StringDict as a Starlark value with attributes.
// This allows access like `registry.http(...)` in Starlark code.
type moduleValue struct {
	name  string
	attrs starlark.StringDict
}

var (
	_ starlark.Value    = (*moduleValue)(nil)
	_ starlark.HasAttrs = (*moduleValue)(nil)
)

func (m *moduleValue) String() string        { return m.name }
func (m *moduleValue) Type() string          { return "module" }
func (m *moduleValue) Freeze()               {} // immutable
func (m *moduleValue) Truth() starlark.Bool  { return true }
func (m *moduleValue) Hash() (uint32, error) { return 0, nil }

func (m *moduleValue) Attr(name string) (starlark.Value, error) {
	if v, ok := m.attrs[name]; ok {
		return v, nil
	}
	return nil, starlark.NoSuchAttrError(
		fmt.Sprintf("%s has no attribute %q", m.name, name),
	)
}

func (m *moduleValue) AttrNames() []string {
	names := make([]string, 0, len(m.attrs))
	for name := range m.attrs {
		names = append(names, name)
	}
	return names
}
