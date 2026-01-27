package builtins

import (
	"errors"
	"fmt"
	"strings"

	"go.starlark.net/starlark"
)

// Fail is a Starlark builtin that raises an error and stops execution.
//
// Usage in Starlark:
//
//	if not valid:
//	    fail("Invalid configuration: missing required field 'url'")
//	fail("Error:", "something went wrong")
var Fail = starlark.NewBuiltin("fail", failImpl)

// FailError is the error type returned by the fail() builtin.
// It can be used to distinguish fail() errors from other Starlark errors.
type FailError struct {
	Message string
}

func (e *FailError) Error() string {
	return e.Message
}

// IsFailError checks if an error was caused by the fail() builtin.
func IsFailError(err error) bool {
	var failErr *FailError
	return errors.As(err, &failErr)
}

func failImpl(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(kwargs) > 0 {
		return nil, fmt.Errorf("fail: unexpected keyword arguments")
	}

	// Build the message from all arguments
	var parts []string
	for _, arg := range args {
		parts = append(parts, formatFailValue(arg))
	}

	msg := strings.Join(parts, " ")
	if msg == "" {
		msg = "fail() called"
	}

	return nil, &FailError{Message: msg}
}

// formatFailValue converts a Starlark value to a string for the error message.
func formatFailValue(v starlark.Value) string {
	// For strings, return the raw value without quotes
	if s, ok := starlark.AsString(v); ok {
		return s
	}
	// For other types, use the String() representation
	return v.String()
}
