// Package starlark provides a reusable Starlark interpreter with extension points.
//
// This package wraps go.starlark.net to provide:
//   - A configurable interpreter with module and builtin registration
//   - File loading with support for Starlark's load() statement
//   - Cycle detection and caching for loaded modules
//   - Go <-> Starlark type conversion utilities
//   - Rich error formatting with source context
//
// # Basic Usage
//
//	interp := starlark.New(
//	    starlark.WithModule("mymodule", myModuleDict),
//	    starlark.WithBuiltin("helper", helperFn),
//	)
//	globals, err := interp.ExecFile("config.star")
//
// # Adding Custom Modules
//
// Modules are Starlark dictionaries that become accessible via module.function() syntax:
//
//	module := starlark.StringDict{
//	    "greet": starlark.NewBuiltin("greet", greetImpl),
//	}
//	interp := starlark.New(starlark.WithModule("hello", module))
//
// # Adding Global Builtins
//
// Builtins are functions available at the top level:
//
//	fn := starlark.NewBuiltin("glob", globImpl)
//	interp := starlark.New(starlark.WithBuiltin("glob", fn))
//
// # Error Handling
//
// The package provides rich error messages with source location context:
//
//	globals, err := interp.ExecFile("config.star")
//	if err != nil {
//	    // Error includes file:line:col and source snippet
//	    fmt.Println(err)
//	}
package starlark
