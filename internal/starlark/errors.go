package starlark

import (
	"errors"
	"fmt"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// Sentinel errors for Starlark operations.
var (
	// ErrCyclicLoad indicates a circular dependency in load() statements.
	ErrCyclicLoad = errors.New("cyclic load detected")

	// ErrModuleNotFound indicates the requested module file does not exist.
	ErrModuleNotFound = errors.New("module not found")

	// ErrInvalidType indicates a type conversion error.
	ErrInvalidType = errors.New("invalid type")
)

// Error wraps a Starlark error with source context.
type Error struct {
	// Pos is the source position where the error occurred.
	Pos syntax.Position

	// Message is the error message.
	Message string

	// Cause is the underlying error, if any.
	Cause error

	// Source is the source line content, if available.
	Source string
}

// Error implements the error interface.
func (e *Error) Error() string {
	var b strings.Builder

	// Write position if available
	if e.Pos.IsValid() {
		fmt.Fprintf(&b, "%s:%d:%d: ", e.Pos.Filename(), e.Pos.Line, e.Pos.Col)
	}

	// Write message
	b.WriteString(e.Message)

	// Write source snippet if available
	if e.Source != "" {
		b.WriteString("\n  ")
		b.WriteString(strings.TrimSpace(e.Source))
		if e.Pos.Col > 0 {
			b.WriteString("\n  ")
			b.WriteString(strings.Repeat(" ", int(e.Pos.Col-1)))
			b.WriteString("^")
		}
	}

	return b.String()
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Cause
}

// WrapError wraps a Starlark error with source context when possible.
func WrapError(err error) error {
	if err == nil {
		return nil
	}

	// Handle Starlark evaluation errors
	var evalErr *starlark.EvalError
	if errors.As(err, &evalErr) {
		return &Error{
			Message: evalErr.Msg,
			Cause:   err,
		}
	}

	// Handle syntax errors
	var syntaxErr syntax.Error
	if errors.As(err, &syntaxErr) {
		return &Error{
			Pos:     syntaxErr.Pos,
			Message: syntaxErr.Msg,
			Cause:   err,
		}
	}

	// Return original error if not a Starlark-specific error
	return err
}

// FormatError formats any error with improved readability.
// For Starlark errors, this provides source context.
// For other errors, this returns the standard error string.
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	// Check if it's already our wrapped error
	var starlarkErr *Error
	if errors.As(err, &starlarkErr) {
		return starlarkErr.Error()
	}

	// Format Starlark evaluation errors nicely
	var evalErr *starlark.EvalError
	if errors.As(err, &evalErr) {
		return formatEvalError(evalErr)
	}

	// Format syntax errors
	var syntaxErr syntax.Error
	if errors.As(err, &syntaxErr) {
		return fmt.Sprintf("%s:%d:%d: %s", syntaxErr.Pos.Filename(), syntaxErr.Pos.Line, syntaxErr.Pos.Col, syntaxErr.Msg)
	}

	return err.Error()
}

// formatEvalError formats an evaluation error with its backtrace.
func formatEvalError(err *starlark.EvalError) string {
	var b strings.Builder

	b.WriteString("error: ")
	b.WriteString(err.Msg)
	b.WriteString("\n")

	// Include backtrace
	backtrace := err.Backtrace()
	if backtrace != "" {
		b.WriteString("\nBacktrace:\n")
		b.WriteString(backtrace)
	}

	return b.String()
}

// NewTypeError creates a type conversion error.
func NewTypeError(expected, got string) error {
	return fmt.Errorf("%w: expected %s, got %s", ErrInvalidType, expected, got)
}

// NewTypeErrorf creates a formatted type conversion error.
func NewTypeErrorf(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrInvalidType, fmt.Sprintf(format, args...))
}
