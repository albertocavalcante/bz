package starlark

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// --- Sentinel errors ---

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	t.Run("ErrCyclicLoad is distinct", func(t *testing.T) {
		t.Parallel()
		assert.NotEqual(t, ErrCyclicLoad, ErrModuleNotFound)
		assert.NotEqual(t, ErrCyclicLoad, ErrInvalidType)
	})

	t.Run("ErrModuleNotFound is distinct", func(t *testing.T) {
		t.Parallel()
		assert.NotEqual(t, ErrModuleNotFound, ErrCyclicLoad)
		assert.NotEqual(t, ErrModuleNotFound, ErrInvalidType)
	})

	t.Run("ErrInvalidType is distinct", func(t *testing.T) {
		t.Parallel()
		assert.NotEqual(t, ErrInvalidType, ErrCyclicLoad)
		assert.NotEqual(t, ErrInvalidType, ErrModuleNotFound)
	})
}

// --- Error type ---

func TestError_ErrorMessage_WithPosition(t *testing.T) {
	t.Parallel()

	e := &Error{
		Pos:     syntax.MakePosition(ptrTo("test.star"), 10, 5),
		Message: "undefined variable x",
	}

	msg := e.Error()
	assert.Contains(t, msg, "test.star")
	assert.Contains(t, msg, "10")
	assert.Contains(t, msg, "5")
	assert.Contains(t, msg, "undefined variable x")
}

func TestError_ErrorMessage_WithoutPosition(t *testing.T) {
	t.Parallel()

	e := &Error{
		Message: "something went wrong",
	}

	msg := e.Error()
	assert.Equal(t, "something went wrong", msg)
}

func TestError_ErrorMessage_WithSource(t *testing.T) {
	t.Parallel()

	e := &Error{
		Pos:     syntax.MakePosition(ptrTo("test.star"), 5, 3),
		Message: "invalid syntax",
		Source:  "x = 1 +",
	}

	msg := e.Error()
	assert.Contains(t, msg, "test.star:5:3")
	assert.Contains(t, msg, "invalid syntax")
	assert.Contains(t, msg, "x = 1 +")
	assert.Contains(t, msg, "^") // caret indicator
}

func TestError_ErrorMessage_WithSourceNoCol(t *testing.T) {
	t.Parallel()

	e := &Error{
		Pos:     syntax.MakePosition(ptrTo("test.star"), 5, 0),
		Message: "error here",
		Source:  "some line",
	}

	msg := e.Error()
	assert.Contains(t, msg, "some line")
	// Col is 0, so no caret
	assert.NotContains(t, msg, "^")
}

func TestError_Unwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("root cause")
	e := &Error{
		Message: "wrapped",
		Cause:   cause,
	}

	assert.Equal(t, cause, e.Unwrap())
}

func TestError_Unwrap_Nil(t *testing.T) {
	t.Parallel()

	e := &Error{
		Message: "no cause",
	}

	assert.Nil(t, e.Unwrap())
}

func TestError_ErrorsIs(t *testing.T) {
	t.Parallel()

	cause := ErrModuleNotFound
	e := &Error{
		Message: "module not found: foo.star",
		Cause:   cause,
	}

	assert.True(t, errors.Is(e, ErrModuleNotFound))
	assert.False(t, errors.Is(e, ErrCyclicLoad))
}

func TestError_ErrorsAs(t *testing.T) {
	t.Parallel()

	e := &Error{
		Message: "test error",
	}

	var target *Error
	assert.True(t, errors.As(e, &target))
	assert.Equal(t, "test error", target.Message)
}

// --- WrapError ---

func TestWrapError_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, WrapError(nil))
}

func TestWrapError_RegularError(t *testing.T) {
	t.Parallel()
	err := errors.New("plain error")
	wrapped := WrapError(err)
	// Non-Starlark errors pass through unchanged
	assert.Equal(t, err, wrapped)
}

func TestWrapError_EvalError(t *testing.T) {
	t.Parallel()

	// Create a real eval error by executing code that fails at runtime.
	// Note: "undefined variable" errors are resolve.ErrorList, not EvalError.
	// Use a runtime error (division by zero) to trigger EvalError.
	fsys := fstest.MapFS{
		"eval_err.star": &fstest.MapFile{
			Data: []byte(`x = 1 // 0`),
		},
	}

	interp := New()
	_, err := interp.ExecFileWithFS(fsys, "eval_err.star")
	require.Error(t, err)

	// Should be wrapped as our Error type
	var starlarkErr *Error
	require.True(t, errors.As(err, &starlarkErr))
	assert.NotEmpty(t, starlarkErr.Message)
	// The cause should be the original eval error
	assert.NotNil(t, starlarkErr.Cause)
}

func TestWrapError_SyntaxError(t *testing.T) {
	t.Parallel()

	// Create a real syntax error
	fsys := fstest.MapFS{
		"syntax_err.star": &fstest.MapFile{
			Data: []byte(`x = 1 +`),
		},
	}

	interp := New()
	_, err := interp.ExecFileWithFS(fsys, "syntax_err.star")
	require.Error(t, err)

	// Should be wrapped as our Error type
	var starlarkErr *Error
	require.True(t, errors.As(err, &starlarkErr))
	assert.NotEmpty(t, starlarkErr.Message)
}

// --- FormatError ---

func TestFormatError_Nil(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", FormatError(nil))
}

func TestFormatError_RegularError(t *testing.T) {
	t.Parallel()
	err := errors.New("plain error")
	assert.Equal(t, "plain error", FormatError(err))
}

func TestFormatError_WrappedError(t *testing.T) {
	t.Parallel()

	e := &Error{
		Pos:     syntax.MakePosition(ptrTo("config.star"), 3, 1),
		Message: "name error",
	}

	formatted := FormatError(e)
	assert.Contains(t, formatted, "config.star:3:1")
	assert.Contains(t, formatted, "name error")
}

func TestFormatError_EvalError(t *testing.T) {
	t.Parallel()

	// Use real Starlark execution to generate an EvalError (runtime error).
	// "undefined" is a resolve error, not an EvalError.
	// Division by zero triggers a real EvalError.
	thread := &starlark.Thread{Name: "test"}
	_, err := starlark.ExecFile(thread, "test.star", []byte("x = 1 // 0"), nil)
	require.Error(t, err)

	formatted := FormatError(err)
	assert.Contains(t, formatted, "error:")
	// Should format with backtrace
	assert.NotEmpty(t, formatted)
}

func TestFormatError_SyntaxError(t *testing.T) {
	t.Parallel()

	// Use real Starlark parsing to generate a syntax error
	thread := &starlark.Thread{Name: "test"}
	_, err := starlark.ExecFile(thread, "bad.star", []byte("x = 1 +"), nil)
	require.Error(t, err)

	formatted := FormatError(err)
	assert.NotEmpty(t, formatted)
}

// --- NewTypeError ---

func TestNewTypeError_Format(t *testing.T) {
	t.Parallel()

	err := NewTypeError("string", "int")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "string")
	assert.Contains(t, err.Error(), "int")
	assert.Contains(t, err.Error(), "expected")
	assert.Contains(t, err.Error(), "got")
	assert.True(t, errors.Is(err, ErrInvalidType))
}

func TestNewTypeError_Various(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		got      string
	}{
		{"string/int", "string", "int"},
		{"bool/string", "bool", "string"},
		{"list or tuple/dict", "list or tuple", "dict"},
		{"non-nil pointer/nil", "non-nil pointer", "nil"},
		{"pointer/string", "pointer", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewTypeError(tt.expected, tt.got)
			assert.True(t, errors.Is(err, ErrInvalidType))
			assert.Contains(t, err.Error(), tt.expected)
			assert.Contains(t, err.Error(), tt.got)
		})
	}
}

// --- NewTypeErrorf ---

func TestNewTypeErrorf_Format(t *testing.T) {
	t.Parallel()

	err := NewTypeErrorf("cannot convert %s to Starlark value", "chan")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot convert chan to Starlark value")
	assert.True(t, errors.Is(err, ErrInvalidType))
}

func TestNewTypeErrorf_WithMultipleArgs(t *testing.T) {
	t.Parallel()

	err := NewTypeErrorf("integer %d overflows %s", 999999999999999, "int8")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "999999999999999")
	assert.Contains(t, err.Error(), "int8")
	assert.True(t, errors.Is(err, ErrInvalidType))
}

// --- Integration: errors from actual Starlark execution ---

func TestError_FromRealExecution_UndefinedVar(t *testing.T) {
	t.Parallel()

	// "undefined variable" errors are resolve.ErrorList, not EvalError,
	// so WrapError passes them through unchanged. We just verify the error
	// contains useful information.
	interp := New()
	_, err := interp.ExecSource("<test>", []byte(`x = y + 1`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undefined")
	assert.Contains(t, err.Error(), "y")
}

func TestError_FromRealExecution_TypeMismatch(t *testing.T) {
	t.Parallel()

	interp := New()
	_, err := interp.ExecSource("<test>", []byte(`x = "hello" + 1`))
	require.Error(t, err)

	var starlarkErr *Error
	assert.True(t, errors.As(err, &starlarkErr))
}

func TestError_FromRealExecution_DivisionByZero(t *testing.T) {
	t.Parallel()

	interp := New()
	_, err := interp.ExecSource("<test>", []byte(`x = 1 // 0`))
	require.Error(t, err)

	var starlarkErr *Error
	assert.True(t, errors.As(err, &starlarkErr))
}

func TestError_FromRealExecution_IndexOutOfRange(t *testing.T) {
	t.Parallel()

	interp := New()
	_, err := interp.ExecSource("<test>", []byte(`x = [1, 2, 3][10]`))
	require.Error(t, err)

	var starlarkErr *Error
	assert.True(t, errors.As(err, &starlarkErr))
}

// ptrTo is a helper to create a pointer to a string (needed for MakePosition).
func ptrTo(s string) *string {
	return &s
}
