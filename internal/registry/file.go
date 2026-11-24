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
	"strings"

	"github.com/albertocavalcante/bz/internal/coord"
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
	if err := coord.ValidateModuleName(module); err != nil {
		return nil, err
	}
	metaPath, err := r.path(MetadataPath(module))
	if err != nil {
		return nil, err
	}

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
	if err := coord.Validate(module, version); err != nil {
		return nil, err
	}

	// First check if module exists
	modulePath, err := r.path(ModulePath(module))
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(modulePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrModuleNotFound
		}
		return nil, fmt.Errorf("stat module: %w", err)
	}

	// Then check for version
	bazelPath, err := r.path(ModuleBazelPath(module, version))
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(bazelPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("read MODULE.bazel: %w", err)
	}

	return data, nil
}

// GetSource fetches the source.json content for a specific version.
func (r *FileRegistry) GetSource(ctx context.Context, module, version string) ([]byte, error) {
	if err := coord.Validate(module, version); err != nil {
		return nil, err
	}

	// First check if module exists
	modulePath, err := r.path(ModulePath(module))
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(modulePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrModuleNotFound
		}
		return nil, fmt.Errorf("stat module: %w", err)
	}

	// Then check for version
	sourcePath, err := r.path(SourcePath(module, version))
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("read source.json: %w", err)
	}

	return data, nil
}

// path joins a slash-separated registry path onto the registry root and
// verifies that the result stays inside the root's modules directory.
func (r *FileRegistry) path(rel string) (string, error) {
	if err := checkModulesRel(rel); err != nil {
		return "", err
	}
	return coord.Join(filepath.Join(r.root, ModulesDir), filepath.FromSlash(strings.TrimPrefix(rel, ModulesDir+"/")))
}

// checkModulesRel verifies that a registry path built by the path helpers
// (which clean their result) still lies under the modules directory and has
// no empty, "." or ".." segments. It is a second layer behind coordinate
// validation.
func checkModulesRel(rel string) error {
	if !strings.HasPrefix(rel, ModulesDir+"/") {
		return fmt.Errorf("%w: %q is not under %s/", coord.ErrEscapesBase, rel, ModulesDir)
	}
	for seg := range strings.SplitSeq(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("%w: %q", coord.ErrEscapesBase, rel)
		}
	}
	return nil
}

// Verify FileRegistry implements Registry and SourceGetter.
var _ Registry = (*FileRegistry)(nil)
var _ SourceGetter = (*FileRegistry)(nil)
