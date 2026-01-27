// Package value provides helper utilities for implementing custom Starlark values.
//
// This package simplifies the creation of custom Starlark values for the bz CLI
// by providing:
//
//   - Base: A common implementation for custom Starlark values that provides
//     default implementations of String(), Type(), Freeze(), Truth(), and Hash().
//
//   - AttrAccessor: A helper for implementing the starlark.HasAttrs interface,
//     making it easy to expose attributes on custom values.
//
//   - Unpacker: A helper for unpacking Starlark function arguments to Go types,
//     with support for required and optional arguments.
//
//   - Type conversion utilities: Functions for converting between Go and Starlark
//     types, including ToStarlark, FromStarlark, and various MustX extractors.
//
//   - WrapFunc: A helper for creating Starlark builtin functions from Go functions
//     using struct tags for argument parsing.
//
// # Example Usage
//
// Creating a custom Starlark value:
//
//	type MyValue struct {
//	    value.Base
//	    value.AttrAccessor
//	    data string
//	}
//
//	func NewMyValue(data string) *MyValue {
//	    v := &MyValue{
//	        Base: value.NewBase("my_value"),
//	        data: data,
//	    }
//	    v.AttrAccessor = value.NewAttrAccessor()
//	    v.Set("data", starlark.String(data))
//	    return v
//	}
//
// Creating a builtin function:
//
//	type FetchArgs struct {
//	    URL  string `starlark:"url,required"`
//	    Auth string `starlark:"auth"`
//	}
//
//	builtin := value.WrapFunc("fetch", func(thread *starlark.Thread, args FetchArgs) (starlark.Value, error) {
//	    // Implementation
//	})
package value
