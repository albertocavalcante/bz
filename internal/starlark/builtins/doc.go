// Package builtins provides common Starlark builtin functions for the bz CLI.
//
// This package contains Bazel-inspired builtins that can be used in Starlark
// scripts processed by bz. The builtins include:
//
//   - glob: Pattern matching for module names using doublestar patterns
//   - env: Access to environment variables with optional defaults
//   - print: Output function (customizable per interpreter)
//   - fail: Error raising function for validation failures
//   - struct: Helper functions for creating Starlark structs
//
// Usage:
//
//	globals := builtins.All()
//	// Add to starlark.ExecFile globals or merge with other builtins
//
// Individual builtins can also be accessed directly:
//
//	globals := starlark.StringDict{
//	    "glob": builtins.Glob,
//	    "env":  builtins.Env,
//	}
package builtins
