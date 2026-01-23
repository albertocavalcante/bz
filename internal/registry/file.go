package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// FileRegistry implements Registry for local filesystem registries.
type FileRegistry struct {
	root string
}

// NewFileRegistry creates a new filesystem-based registry.
func NewFileRegistry(root string) *FileRegistry {
	return &FileRegistry{root: root}
}

// Type returns the registry type identifier.
func (r *FileRegistry) Type() string {
	return TypeFile
}

// String returns a human-readable representation.
func (r *FileRegistry) String() string {
	return "file://" + r.root
}

// ListModules returns all module names in the registry.
func (r *FileRegistry) ListModules(ctx context.Context) ([]string, error) {
	modulesDir := filepath.Join(r.root, ModulesDir)

	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: modules directory not found at %s", ErrListingNotSupported, modulesDir)
		}
		return nil, fmt.Errorf("read modules directory: %w", err)
	}

	var modules []string
	for _, entry := range entries {
		// Only include directories (modules), skip files like index.json
		if entry.IsDir() {
			modules = append(modules, entry.Name())
		}
	}

	sort.Strings(modules)
	return modules, nil
}

// GetMetadata fetches the metadata.json for a module.
func (r *FileRegistry) GetMetadata(ctx context.Context, module string) (*Metadata, error) {
	metaPath := filepath.Join(r.root, MetadataPath(module))

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrModuleNotFound
		}
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}

	return &meta, nil
}

// GetModuleBazel fetches the MODULE.bazel content for a specific version.
func (r *FileRegistry) GetModuleBazel(ctx context.Context, module, version string) ([]byte, error) {
	// First check if module exists
	modulePath := filepath.Join(r.root, ModulePath(module))
	if _, err := os.Stat(modulePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrModuleNotFound
		}
		return nil, fmt.Errorf("stat module: %w", err)
	}

	// Then check for version
	bazelPath := filepath.Join(r.root, ModuleBazelPath(module, version))
	data, err := os.ReadFile(bazelPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("read MODULE.bazel: %w", err)
	}

	return data, nil
}

// Verify FileRegistry implements Registry.
var _ Registry = (*FileRegistry)(nil)
