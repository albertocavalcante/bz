package modules

import (
	"sort"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

func TestAll_ReturnsExpectedModules(t *testing.T) {
	t.Parallel()

	modules := All()

	expectedKeys := []string{"registry", "auth", "sync", "config"}
	for _, key := range expectedKeys {
		if _, ok := modules[key]; !ok {
			t.Errorf("All() missing expected module %q", key)
		}
	}

	if len(modules) != len(expectedKeys) {
		t.Errorf("All() returned %d modules, want %d", len(modules), len(expectedKeys))
	}
}

func TestAll_ModulesHaveEntries(t *testing.T) {
	t.Parallel()

	modules := All()

	for name, dict := range modules {
		if len(dict) == 0 {
			t.Errorf("module %q has no entries", name)
		}
	}
}

func TestPredeclared_ReturnsModuleValues(t *testing.T) {
	t.Parallel()

	predeclared := Predeclared()

	expectedModules := []string{"registry", "auth", "sync", "config"}
	for _, name := range expectedModules {
		val, ok := predeclared[name]
		if !ok {
			t.Errorf("Predeclared() missing module %q", name)
			continue
		}

		mv, ok := val.(*moduleValue)
		if !ok {
			t.Errorf("Predeclared()[%q] is %T, want *moduleValue", name, val)
			continue
		}

		if mv.name != name {
			t.Errorf("Predeclared()[%q].name = %q, want %q", name, mv.name, name)
		}
	}

	if len(predeclared) != len(expectedModules) {
		t.Errorf("Predeclared() returned %d entries, want %d", len(predeclared), len(expectedModules))
	}
}

func TestModuleValue_String(t *testing.T) {
	t.Parallel()

	predeclared := Predeclared()

	for name, val := range predeclared {
		if val.String() != name {
			t.Errorf("moduleValue.String() = %q, want %q", val.String(), name)
		}
	}
}

func TestModuleValue_Type(t *testing.T) {
	t.Parallel()

	predeclared := Predeclared()

	for name, val := range predeclared {
		if val.Type() != "module" {
			t.Errorf("moduleValue[%q].Type() = %q, want %q", name, val.Type(), "module")
		}
	}
}

func TestModuleValue_Truth(t *testing.T) {
	t.Parallel()

	predeclared := Predeclared()

	for name, val := range predeclared {
		mv := val.(*moduleValue)
		if mv.Truth() != starlark.True {
			t.Errorf("moduleValue[%q].Truth() = %v, want true", name, mv.Truth())
		}
	}
}

func TestModuleValue_Hash(t *testing.T) {
	t.Parallel()

	predeclared := Predeclared()

	for name, val := range predeclared {
		mv := val.(*moduleValue)
		_, err := mv.Hash()
		if err == nil {
			t.Errorf("moduleValue[%q].Hash() should return error, got nil", name)
			continue
		}
		if !strings.Contains(err.Error(), "unhashable") {
			t.Errorf("moduleValue[%q].Hash() error = %q, want error containing 'unhashable'", name, err.Error())
		}
	}
}

func TestModuleValue_Attr_Exists(t *testing.T) {
	t.Parallel()

	// Use a known attribute from each module to verify Attr() works.
	tests := []struct {
		module string
		attr   string
	}{
		{"registry", "http"},
		{"auth", "basic"},
		{"sync", "workflow"},
		{"config", "defaults"},
	}

	predeclared := Predeclared()

	for _, tt := range tests {
		mv := predeclared[tt.module].(*moduleValue)

		got, err := mv.Attr(tt.attr)
		if err != nil {
			t.Errorf("moduleValue[%q].Attr(%q) returned unexpected error: %v", tt.module, tt.attr, err)
			continue
		}
		if got == nil {
			t.Errorf("moduleValue[%q].Attr(%q) returned nil", tt.module, tt.attr)
			continue
		}

		// Verify the returned value is a starlark.Builtin (all module entries are builtins).
		if _, ok := got.(*starlark.Builtin); !ok {
			t.Errorf("moduleValue[%q].Attr(%q) returned %T, want *starlark.Builtin", tt.module, tt.attr, got)
		}
	}
}

func TestModuleValue_Attr_NotExists(t *testing.T) {
	t.Parallel()

	predeclared := Predeclared()

	for name, val := range predeclared {
		mv := val.(*moduleValue)
		_, err := mv.Attr("nonexistent_attr")
		if err == nil {
			t.Errorf("moduleValue[%q].Attr(\"nonexistent_attr\") should return error, got nil", name)
			continue
		}

		// The error should be a NoSuchAttrError containing the module name and the bad attribute.
		errMsg := err.Error()
		if !strings.Contains(errMsg, name) {
			t.Errorf("moduleValue[%q].Attr error should mention module name: %q", name, errMsg)
		}
		if !strings.Contains(errMsg, "nonexistent_attr") {
			t.Errorf("moduleValue[%q].Attr error should mention attribute name: %q", name, errMsg)
		}
	}
}

func TestModuleValue_AttrNames(t *testing.T) {
	t.Parallel()

	modules := All()
	predeclared := Predeclared()

	for name, dict := range modules {
		mv := predeclared[name].(*moduleValue)

		attrNames := mv.AttrNames()

		// Build the expected set of names from the dict keys.
		expectedNames := make([]string, 0, len(dict))
		for k := range dict {
			expectedNames = append(expectedNames, k)
		}

		sort.Strings(attrNames)
		sort.Strings(expectedNames)

		if len(attrNames) != len(expectedNames) {
			t.Errorf("moduleValue[%q].AttrNames() returned %d names, want %d", name, len(attrNames), len(expectedNames))
			continue
		}

		for i := range attrNames {
			if attrNames[i] != expectedNames[i] {
				t.Errorf("moduleValue[%q].AttrNames()[%d] = %q, want %q", name, i, attrNames[i], expectedNames[i])
			}
		}
	}
}
