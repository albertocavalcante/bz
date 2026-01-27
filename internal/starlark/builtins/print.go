package builtins

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"
)

// PrintFunc is a function type for custom print handlers.
// It receives the thread context and the formatted message.
type PrintFunc func(thread *starlark.Thread, msg string)

// printFuncKey is the thread-local key for storing a custom print function.
const printFuncKey = "print_func"

// SetPrintFunc sets a custom print function for a Starlark thread.
// If not set, print output goes to stdout.
func SetPrintFunc(thread *starlark.Thread, fn PrintFunc) {
	thread.SetLocal(printFuncKey, fn)
}

// GetPrintFunc retrieves the custom print function from a Starlark thread.
func GetPrintFunc(thread *starlark.Thread) PrintFunc {
	if v := thread.Local(printFuncKey); v != nil {
		if fn, ok := v.(PrintFunc); ok {
			return fn
		}
	}
	return nil
}

// Print is a Starlark builtin that outputs text.
// It can be customized per-thread using SetPrintFunc.
//
// Usage in Starlark:
//
//	print("Hello, world!")
//	print("Value:", x, "Count:", n)
var Print = starlark.NewBuiltin("print", printImpl)

func printImpl(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Build the message from all arguments
	parts := make([]string, 0, len(args)+len(kwargs))
	for _, arg := range args {
		parts = append(parts, formatValue(arg))
	}

	// Append kwargs as key=value pairs
	for _, kv := range kwargs {
		key := string(kv[0].(starlark.String))
		parts = append(parts, fmt.Sprintf("%s=%s", key, formatValue(kv[1])))
	}

	msg := strings.Join(parts, " ")

	// Use custom print function if set, otherwise use fmt.Println
	if fn := GetPrintFunc(thread); fn != nil {
		fn(thread, msg)
	} else {
		fmt.Println(msg)
	}

	return starlark.None, nil
}

// formatValue converts a Starlark value to a string for printing.
func formatValue(v starlark.Value) string {
	// For strings, return the raw value without quotes
	if s, ok := starlark.AsString(v); ok {
		return s
	}
	// For other types, use the String() representation
	return v.String()
}
