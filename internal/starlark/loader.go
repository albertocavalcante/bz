package starlark

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// Loader handles Starlark file loading with support for load() statements.
type Loader struct {
	// mu protects all mutable state
	mu sync.Mutex

	// cache stores already-loaded modules by absolute path
	cache map[string]*cacheEntry

	// loading tracks modules currently being loaded (for cycle detection)
	loading map[string]bool

	// searchPaths are directories to search for imports
	searchPaths []string

	// fsys is an optional filesystem for loading files (useful for testing)
	fsys fs.FS

	// fsysRoot is the root path that fsys is mounted at
	fsysRoot string

	// predeclared are globals available to all loaded files
	predeclared starlark.StringDict

	// fileOptions controls Starlark parsing and execution options
	fileOptions *syntax.FileOptions
}

// cacheEntry holds a loaded module and any error from loading.
type cacheEntry struct {
	globals starlark.StringDict
	err     error
}

// LoaderOption configures a Loader.
type LoaderOption func(*Loader)

// WithSearchPaths adds directories to search for imports.
func WithSearchPaths(paths ...string) LoaderOption {
	return func(l *Loader) {
		l.searchPaths = append(l.searchPaths, paths...)
	}
}

// WithFS sets a filesystem for loading files.
func WithFS(fsys fs.FS, root string) LoaderOption {
	return func(l *Loader) {
		l.fsys = fsys
		l.fsysRoot = root
	}
}

// WithLoaderPredeclared sets predeclared globals for loaded files.
func WithLoaderPredeclared(predeclared starlark.StringDict) LoaderOption {
	return func(l *Loader) {
		l.predeclared = predeclared
	}
}

// WithLoaderFileOptions sets the Starlark file options for loaded files.
func WithLoaderFileOptions(opts *syntax.FileOptions) LoaderOption {
	return func(l *Loader) {
		l.fileOptions = opts
	}
}

// NewLoader creates a new file loader.
func NewLoader(opts ...LoaderOption) *Loader {
	l := &Loader{
		cache:       make(map[string]*cacheEntry),
		loading:     make(map[string]bool),
		predeclared: make(starlark.StringDict),
		fileOptions: defaultFileOptions,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Load implements the starlark.Thread load function.
// It loads a module and returns the specified bindings.
//
// Supports:
//   - Relative imports: load("./common.star", "helper")
//   - Search path imports: load("lib/utils.star", "util")
func (l *Loader) Load(thread *starlark.Thread, module string) (starlark.StringDict, error) {
	// Resolve the module path
	absPath, err := l.resolvePath(thread, module)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", module, err)
	}

	return l.loadFile(absPath)
}

// resolvePath resolves a module name to an absolute path.
func (l *Loader) resolvePath(thread *starlark.Thread, module string) (string, error) {
	// Get the directory of the current file being executed
	currentFile := ""
	if callFrame := thread.CallFrame(0); callFrame.Pos.Filename() != "" {
		currentFile = callFrame.Pos.Filename()
	}

	// Handle relative imports
	if strings.HasPrefix(module, "./") || strings.HasPrefix(module, "../") {
		if currentFile == "" {
			return "", fmt.Errorf("relative import %q with no current file", module)
		}
		baseDir := filepath.Dir(currentFile)
		return filepath.Abs(filepath.Join(baseDir, module))
	}

	// Try search paths
	for _, searchPath := range l.searchPaths {
		candidate := filepath.Join(searchPath, module)
		if l.fileExists(candidate) {
			return filepath.Abs(candidate)
		}
	}

	// Try relative to current file's directory
	if currentFile != "" {
		baseDir := filepath.Dir(currentFile)
		candidate := filepath.Join(baseDir, module)
		if l.fileExists(candidate) {
			return filepath.Abs(candidate)
		}
	}

	// Try as absolute path or relative to working directory
	if filepath.IsAbs(module) {
		if l.fileExists(module) {
			return module, nil
		}
	} else {
		candidate, err := filepath.Abs(module)
		if err == nil && l.fileExists(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrModuleNotFound, module)
}

// fileExists checks if a file exists.
func (l *Loader) fileExists(path string) bool {
	if l.fsys != nil {
		// Convert absolute path to relative for fs.FS
		relPath, err := l.toFSPath(path)
		if err != nil {
			return false
		}
		_, err = fs.Stat(l.fsys, relPath)
		return err == nil
	}
	_, err := os.Stat(path)
	return err == nil
}

// toFSPath converts an absolute path to a path relative to the fsys root.
func (l *Loader) toFSPath(absPath string) (string, error) {
	if l.fsysRoot == "" {
		// Assume the path is already relative or use as-is
		return strings.TrimPrefix(absPath, "/"), nil
	}
	rel, err := filepath.Rel(l.fsysRoot, absPath)
	if err != nil {
		return "", err
	}
	// fs.FS uses forward slashes
	return filepath.ToSlash(rel), nil
}

// loadFile loads a file by absolute path, using cache and cycle detection.
func (l *Loader) loadFile(absPath string) (starlark.StringDict, error) {
	l.mu.Lock()

	// Check cache
	if entry, ok := l.cache[absPath]; ok {
		l.mu.Unlock()
		return entry.globals, entry.err
	}

	// Check for cycles
	if l.loading[absPath] {
		l.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrCyclicLoad, absPath)
	}

	// Mark as loading
	l.loading[absPath] = true
	l.mu.Unlock()

	// Load the file
	globals, err := l.doLoad(absPath)

	// Update cache
	l.mu.Lock()
	l.cache[absPath] = &cacheEntry{globals: globals, err: err}
	delete(l.loading, absPath)
	l.mu.Unlock()

	return globals, err
}

// doLoad performs the actual file loading.
func (l *Loader) doLoad(absPath string) (starlark.StringDict, error) {
	// Read file content
	src, err := l.readFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", absPath, err)
	}

	// Create thread for this load
	thread := &starlark.Thread{
		Name: absPath,
		Load: l.Load,
	}

	// Execute the file
	globals, err := starlark.ExecFileOptions(l.fileOptions, thread, absPath, src, l.predeclared)
	if err != nil {
		return nil, WrapError(err)
	}

	return globals, nil
}

// readFile reads a file's content.
func (l *Loader) readFile(absPath string) ([]byte, error) {
	if l.fsys != nil {
		relPath, err := l.toFSPath(absPath)
		if err != nil {
			return nil, err
		}
		return fs.ReadFile(l.fsys, relPath)
	}
	return os.ReadFile(absPath)
}

// ClearCache clears the module cache.
func (l *Loader) ClearCache() {
	l.mu.Lock()
	l.cache = make(map[string]*cacheEntry)
	l.mu.Unlock()
}
