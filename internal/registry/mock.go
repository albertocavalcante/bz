package registry

import (
	"context"
)

// MockRegistry is a test implementation of Registry.
type MockRegistry struct {
	Modules  map[string]*Metadata
	TypeName string
}

// NewMockRegistry creates a new MockRegistry with the given modules.
func NewMockRegistry(modules map[string]*Metadata) *MockRegistry {
	return &MockRegistry{
		Modules:  modules,
		TypeName: "mock",
	}
}

// GetMetadata returns the metadata for the given module.
func (m *MockRegistry) GetMetadata(_ context.Context, module string) (*Metadata, error) {
	if meta, ok := m.Modules[module]; ok {
		return meta, nil
	}
	return nil, ErrModuleNotFound
}

// GetModuleBazel returns the MODULE.bazel content for the given module and version.
func (m *MockRegistry) GetModuleBazel(_ context.Context, module, version string) ([]byte, error) {
	if _, ok := m.Modules[module]; !ok {
		return nil, ErrModuleNotFound
	}
	return []byte("module(name = \"" + module + "\", version = \"" + version + "\")"), nil
}

// ListModules returns all available module names.
func (m *MockRegistry) ListModules(_ context.Context) ([]string, error) {
	names := make([]string, 0, len(m.Modules))
	for name := range m.Modules {
		names = append(names, name)
	}
	return names, nil
}

// Type returns the registry type.
func (m *MockRegistry) Type() string {
	return m.TypeName
}

// String returns a string representation.
func (m *MockRegistry) String() string {
	return "mock://test"
}
