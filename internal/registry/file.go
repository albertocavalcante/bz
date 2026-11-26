package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/albertocavalcante/bz/internal/coord"
)

// FileRegistry implements Registry for local filesystem registries.
type FileRegistry struct {
	root    string
	bounded bool
}

// NewFileRegistry creates a new filesystem-based registry.
func NewFileRegistry(root string) *FileRegistry {
	return &FileRegistry{root: root}
}

// NewCacheRegistry creates a local registry with bounded reads for cached data.
func NewCacheRegistry(root string) *FileRegistry {
	return &FileRegistry{root: root, bounded: true}
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
	data, err := r.readFile(MetadataPath(module))
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
	if err := r.stat(ModulePath(module)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrModuleNotFound
		}
		return nil, fmt.Errorf("stat module: %w", err)
	}

	// Then check for version
	data, err := r.readFile(ModuleBazelPath(module, version))
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
	if err := r.stat(ModulePath(module)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrModuleNotFound
		}
		return nil, fmt.Errorf("stat module: %w", err)
	}

	// Then check for version
	data, err := r.readFile(SourcePath(module, version))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("read source.json: %w", err)
	}

	return data, nil
}

// GetVersionFile reads a bounded auxiliary file from the version directory.
func (r *FileRegistry) GetVersionFile(_ context.Context, module, version, name string) ([]byte, error) {
	rel, err := versionFilePath(module, version, name)
	if err != nil {
		return nil, err
	}
	bounded := *r
	bounded.bounded = true
	return bounded.readFile(rel)
}

// modulesName checks a slash-separated registry path ("modules/...") and
// returns it relative to the modules directory. This is the lexical first
// layer; reads then go through os.Root.
func (r *FileRegistry) modulesName(rel string) (string, error) {
	if err := checkModulesRel(rel); err != nil {
		return "", err
	}
	name := filepath.FromSlash(strings.TrimPrefix(rel, ModulesDir+"/"))
	if _, err := coord.Join(filepath.Join(r.root, ModulesDir), name); err != nil {
		return "", err
	}
	return name, nil
}

// openModules opens the registry root and then descends into its modules
// directory, both as os.Root, so neither a "modules" entry that is itself a
// symlink leaving the registry nor symlinks below it can redirect a read
// outside the registry. Symlinks whose targets stay inside the registry
// root still resolve. The caller closes the returned root; the registry
// root handle is closed before returning.
func (r *FileRegistry) openModules() (*os.Root, error) {
	rootPath := r.root
	if r.bounded {
		rootPath = registryDirectoryPath(rootPath)
	}
	base, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer base.Close()
	modulesPath := ModulesDir
	if r.bounded {
		modulesPath = registryDirectoryPath(modulesPath)
	}
	return base.OpenRoot(modulesPath)
}

// readFile reads a registry file confined to the modules directory.
func (r *FileRegistry) readFile(rel string) ([]byte, error) {
	name, err := r.modulesName(rel)
	if err != nil {
		return nil, err
	}
	root, err := r.openModules()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if !r.bounded {
		return root.ReadFile(name)
	}
	return ReadBounded(root, name)
}

// MaxFileBytes is the maximum size accepted for a local registry file.
const MaxFileBytes = maxFetchResponseBytes

// ReadBounded reads a confined regular file with the registry fetch size limit.
func ReadBounded(root *os.Root, name string) ([]byte, error) {
	file, err := root.OpenFile(name, registryReadFlags(), 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("registry file is not a regular file")
	}
	if info.Size() > MaxFileBytes {
		return nil, fmt.Errorf("registry file exceeds %d-byte limit", MaxFileBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("registry file exceeds %d-byte limit", MaxFileBytes)
	}
	return data, nil
}

// DirectoryPath forces directory traversal before opening a directory.
func DirectoryPath(name string) string { return registryDirectoryPath(name) }

// stat checks that a registry entry exists inside the modules directory.
func (r *FileRegistry) stat(rel string) error {
	name, err := r.modulesName(rel)
	if err != nil {
		return err
	}
	root, err := r.openModules()
	if err != nil {
		return err
	}
	defer root.Close()
	_, err = root.Stat(name)
	return err
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
