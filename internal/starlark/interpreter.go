package starlark

import (
	"io/fs"
	"os"
	"path/filepath"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// defaultFileOptions provides the default Starlark file options.
// These enable useful Starlark features while maintaining reasonable safety.
var defaultFileOptions = &syntax.FileOptions{
	Set:             true, // allow set() builtin
	While:           true, // allow while loops
	TopLevelControl: true, // allow if/for/while at top level
	GlobalReassign:  true, // allow reassignment to top-level names
	Recursion:       true, // allow recursive functions
}

// Interpreter wraps go.starlark.net with extension points for adding modules and builtins.
//
// Design goals:
//   - Reusable across different config use cases
//   - Easy to add custom modules (like "registry", "sync", "auth")
//   - Easy to add global builtins (like "glob", "env")
//   - Good error messages with source locations
type Interpreter struct {
	// predeclared holds all global values (modules, builtins, and other values)
	predeclared starlark.StringDict

	// thread is the Starlark execution thread
	thread *starlark.Thread

	// loader handles load() statements
	loader *Loader

	// searchPaths for module resolution
	searchPaths []string

	// fileOptions controls Starlark parsing and execution options
	fileOptions *syntax.FileOptions
}

// Option configures an Interpreter.
type Option func(*Interpreter)

// WithModule adds a Starlark module accessible as module.function().
//
// Example:
//
//	registryModule := starlark.StringDict{
//	    "search": starlark.NewBuiltin("search", searchImpl),
//	    "get":    starlark.NewBuiltin("get", getImpl),
//	}
//	interp := starlark.New(starlark.WithModule("registry", registryModule))
//
// In Starlark:
//
//	result = registry.search("rules_go")
func WithModule(name string, members starlark.StringDict) Option {
	return func(i *Interpreter) {
		// Create a struct module from the dict
		module := starlarkstruct.FromStringDict(starlark.String(name), members)
		i.predeclared[name] = module
	}
}

// WithBuiltin adds a global builtin function.
//
// Example:
//
//	globFn := starlark.NewBuiltin("glob", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
//	    // implementation
//	})
//	interp := starlark.New(starlark.WithBuiltin("glob", globFn))
//
// In Starlark:
//
//	files = glob("*.go")
func WithBuiltin(name string, fn *starlark.Builtin) Option {
	return func(i *Interpreter) {
		i.predeclared[name] = fn
	}
}

// WithPredeclared adds a predeclared value to globals.
// This can be any Starlark value (string, int, dict, struct, etc.).
//
// Example:
//
//	interp := starlark.New(starlark.WithPredeclared("VERSION", starlark.String("1.0.0")))
//
// In Starlark:
//
//	print(VERSION)  # prints "1.0.0"
func WithPredeclared(name string, value starlark.Value) Option {
	return func(i *Interpreter) {
		i.predeclared[name] = value
	}
}

// WithSearchPath adds a directory to search for imports in load() statements.
//
// Example:
//
//	interp := starlark.New(starlark.WithSearchPath("/path/to/libs"))
//
// In Starlark:
//
//	load("utils.star", "helper")  # searches in /path/to/libs/utils.star
func WithSearchPath(path string) Option {
	return func(i *Interpreter) {
		i.searchPaths = append(i.searchPaths, path)
	}
}

// WithPrint sets a custom print function for the interpreter.
//
// Example:
//
//	interp := starlark.New(starlark.WithPrint(func(thread *starlark.Thread, msg string) {
//	    log.Printf("[starlark] %s", msg)
//	}))
func WithPrint(fn func(thread *starlark.Thread, msg string)) Option {
	return func(i *Interpreter) {
		i.thread.Print = fn
	}
}

// New creates a configured interpreter.
//
// Example:
//
//	interp := starlark.New(
//	    starlark.WithModule("registry", registryModule),
//	    starlark.WithBuiltin("glob", globFn),
//	    starlark.WithPredeclared("DEBUG", starlark.Bool(true)),
//	)
func New(opts ...Option) *Interpreter {
	i := &Interpreter{
		predeclared: make(starlark.StringDict),
		thread:      &starlark.Thread{Name: "main"},
		fileOptions: defaultFileOptions,
	}

	// Apply options
	for _, opt := range opts {
		opt(i)
	}

	// Create loader with search paths and predeclared values
	i.loader = NewLoader(
		WithSearchPaths(i.searchPaths...),
		WithLoaderPredeclared(i.predeclared),
		WithLoaderFileOptions(i.fileOptions),
	)

	// Set up load function on thread
	i.thread.Load = i.loader.Load

	return i
}

// ExecFile executes a Starlark file and returns the global bindings.
//
// The file path can be absolute or relative to the current working directory.
// Returns an error with source location information if execution fails.
//
// Example:
//
//	globals, err := interp.ExecFile("config.star")
//	if err != nil {
//	    log.Fatal(starlark.FormatError(err))
//	}
//	name, _ := globals["name"]
func (i *Interpreter) ExecFile(filename string) (starlark.StringDict, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}

	// Read the file
	src, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	// Execute
	globals, err := starlark.ExecFileOptions(i.fileOptions, i.thread, absPath, src, i.predeclared)
	if err != nil {
		return nil, WrapError(err)
	}

	return globals, nil
}

// ExecFileWithFS executes a Starlark file using an fs.FS.
// This is useful for testing with embedded files or in-memory filesystems.
//
// The fsys parameter provides the filesystem, and fsysRoot specifies
// where in the filesystem the files are rooted (for load() resolution).
//
// Example:
//
//	fsys := fstest.MapFS{
//	    "config.star": &fstest.MapFile{Data: []byte(`x = 1`)},
//	}
//	globals, err := interp.ExecFileWithFS(fsys, "", "config.star")
func (i *Interpreter) ExecFileWithFS(fsys fs.FS, filename string) (starlark.StringDict, error) {
	// Read from the provided filesystem
	src, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return nil, err
	}

	// Create a new loader for this filesystem
	// Use current working directory as root for resolving relative paths
	cwd, _ := os.Getwd()
	loader := NewLoader(
		WithFS(fsys, cwd),
		WithSearchPaths(i.searchPaths...),
		WithLoaderPredeclared(i.predeclared),
		WithLoaderFileOptions(i.fileOptions),
	)

	// Create a thread for this execution
	thread := &starlark.Thread{
		Name:  filename,
		Load:  loader.Load,
		Print: i.thread.Print,
	}

	// Execute
	globals, err := starlark.ExecFileOptions(i.fileOptions, thread, filename, src, i.predeclared)
	if err != nil {
		return nil, WrapError(err)
	}

	return globals, nil
}

// ExecSource executes Starlark source code directly and returns the global bindings.
// The filename parameter is used for error messages.
//
// Example:
//
//	globals, err := interp.ExecSource("<inline>", `x = 1 + 2`)
func (i *Interpreter) ExecSource(filename string, src []byte) (starlark.StringDict, error) {
	globals, err := starlark.ExecFileOptions(i.fileOptions, i.thread, filename, src, i.predeclared)
	if err != nil {
		return nil, WrapError(err)
	}
	return globals, nil
}

// Thread returns the Starlark thread.
// This is useful for advanced use cases like setting thread-local values.
func (i *Interpreter) Thread() *starlark.Thread {
	return i.thread
}

// Predeclared returns the predeclared globals.
// Modifications to the returned dict will affect the interpreter.
func (i *Interpreter) Predeclared() starlark.StringDict {
	return i.predeclared
}

// Call calls a Starlark function with the given arguments.
// This is useful for calling functions defined in executed files.
//
// Example:
//
//	globals, _ := interp.ExecFile("funcs.star")
//	fn := globals["greet"].(*starlark.Function)
//	result, err := interp.Call(fn, starlark.Tuple{starlark.String("world")}, nil)
func (i *Interpreter) Call(fn starlark.Callable, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	result, err := starlark.Call(i.thread, fn, args, kwargs)
	if err != nil {
		return nil, WrapError(err)
	}
	return result, nil
}
