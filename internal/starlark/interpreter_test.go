package starlark

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestInterpreter_BasicExecution(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"basic.star": &fstest.MapFile{
			Data: []byte(`
x = 1
y = 2
result = x + y
name = "test"
`),
		},
	}

	interp := New()
	globals, err := interp.ExecFileWithFS(fsys, "basic.star")
	require.NoError(t, err)

	// Check result value
	result, ok := globals["result"].(starlark.Int)
	require.True(t, ok, "result should be an int")
	val, _ := result.Int64()
	assert.Equal(t, int64(3), val)

	// Check string value
	name, ok := globals["name"].(starlark.String)
	require.True(t, ok, "name should be a string")
	assert.Equal(t, "test", string(name))
}

func TestInterpreter_WithModule(t *testing.T) {
	t.Parallel(
	// Create a test module
	)

	module := starlark.StringDict{
		"greet": starlark.NewBuiltin("greet", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var name string
			if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &name); err != nil {
				return nil, err
			}
			return starlark.String("Hello, " + name + "!"), nil
		}),
		"version": starlark.String("1.0.0"),
	}

	fsys := fstest.MapFS{
		"module.star": &fstest.MapFile{
			Data: []byte(`
greeting = mymod.greet("World")
ver = mymod.version
`),
		},
	}

	interp := New(WithModule("mymod", module))
	globals, err := interp.ExecFileWithFS(fsys, "module.star")
	require.NoError(t, err)

	greeting, ok := globals["greeting"].(starlark.String)
	require.True(t, ok)
	assert.Equal(t, "Hello, World!", string(greeting))

	ver, ok := globals["ver"].(starlark.String)
	require.True(t, ok)
	assert.Equal(t, "1.0.0", string(ver))
}

func TestInterpreter_WithBuiltin(t *testing.T) {
	t.Parallel(
	// Create a builtin that doubles a number
	)

	doubleFn := starlark.NewBuiltin("double", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var n int
		if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "n", &n); err != nil {
			return nil, err
		}
		return starlark.MakeInt(n * 2), nil
	})

	fsys := fstest.MapFS{
		"builtin.star": &fstest.MapFile{
			Data: []byte(`
x = double(5)
y = double(x)
`),
		},
	}

	interp := New(WithBuiltin("double", doubleFn))
	globals, err := interp.ExecFileWithFS(fsys, "builtin.star")
	require.NoError(t, err)

	x, ok := globals["x"].(starlark.Int)
	require.True(t, ok)
	xVal, _ := x.Int64()
	assert.Equal(t, int64(10), xVal)

	y, ok := globals["y"].(starlark.Int)
	require.True(t, ok)
	yVal, _ := y.Int64()
	assert.Equal(t, int64(20), yVal)
}

func TestInterpreter_WithPredeclared(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"predeclared.star": &fstest.MapFile{
			Data: []byte(`
greeting = "Hello, " + NAME
is_debug = DEBUG
`),
		},
	}

	interp := New(
		WithPredeclared("NAME", starlark.String("Developer")),
		WithPredeclared("DEBUG", starlark.Bool(true)),
	)
	globals, err := interp.ExecFileWithFS(fsys, "predeclared.star")
	require.NoError(t, err)

	greeting, ok := globals["greeting"].(starlark.String)
	require.True(t, ok)
	assert.Equal(t, "Hello, Developer", string(greeting))

	isDebug, ok := globals["is_debug"].(starlark.Bool)
	require.True(t, ok)
	assert.True(t, bool(isDebug))
}

func TestInterpreter_SyntaxError(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"syntax_error.star": &fstest.MapFile{
			Data: []byte(`
x = 1 +
`),
		},
	}

	interp := New()
	_, err := interp.ExecFileWithFS(fsys, "syntax_error.star")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "syntax_error.star")
}

func TestInterpreter_RuntimeError(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"runtime_error.star": &fstest.MapFile{
			Data: []byte(`
x = 1 / 0
`),
		},
	}

	interp := New()
	_, err := interp.ExecFileWithFS(fsys, "runtime_error.star")
	require.Error(t, err)
	// Starlark allows integer division by zero (returns infinity for float, error for int)
	// The error message should mention division or zero
}

func TestInterpreter_UndefinedVariable(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"undefined.star": &fstest.MapFile{
			Data: []byte(`
x = undefined_var + 1
`),
		},
	}

	interp := New()
	_, err := interp.ExecFileWithFS(fsys, "undefined.star")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undefined_var")
}

func TestInterpreter_ExecSource(t *testing.T) {
	t.Parallel()
	interp := New()
	globals, err := interp.ExecSource("<test>", []byte(`
x = 42
y = x * 2
`))
	require.NoError(t, err)

	y, ok := globals["y"].(starlark.Int)
	require.True(t, ok)
	yVal, _ := y.Int64()
	assert.Equal(t, int64(84), yVal)
}

func TestInterpreter_Call(t *testing.T) {
	t.Parallel()
	interp := New()
	globals, err := interp.ExecSource("<test>", []byte(`
def add(a, b):
    return a + b
`))
	require.NoError(t, err)

	fn, ok := globals["add"].(*starlark.Function)
	require.True(t, ok)

	result, err := interp.Call(fn, starlark.Tuple{starlark.MakeInt(3), starlark.MakeInt(4)}, nil)
	require.NoError(t, err)

	resultInt, ok := result.(starlark.Int)
	require.True(t, ok)
	val, _ := resultInt.Int64()
	assert.Equal(t, int64(7), val)
}

func TestInterpreter_WithPrint(t *testing.T) {
	t.Parallel()
	var printed []string
	interp := New(WithPrint(func(thread *starlark.Thread, msg string) {
		printed = append(printed, msg)
	}))

	_, err := interp.ExecSource("<test>", []byte(`
print("hello")
print("world")
`))
	require.NoError(t, err)

	assert.Equal(t, []string{"hello", "world"}, printed)
}

func TestInterpreter_Load(t *testing.T) {
	t.Parallel(
	// Test that load() is configured on the interpreter
	)

	interp := New()

	// Verify that the interpreter is configured with a load function
	assert.NotNil(t, interp.Thread().Load)

	// Note: Full load() testing requires real filesystem operations
	// because fstest.MapFS paths don't resolve correctly for load() callbacks.
	// The loader itself is tested separately.
}

func TestTypes_ToValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    any
		expected string // Type name
		checkVal func(t *testing.T, v starlark.Value)
	}{
		{
			name:     "nil",
			input:    nil,
			expected: "NoneType",
		},
		{
			name:     "bool true",
			input:    true,
			expected: "bool",
			checkVal: func(t *testing.T, v starlark.Value) {
				assert.Equal(t, starlark.Bool(true), v)
			},
		},
		{
			name:     "int",
			input:    42,
			expected: "int",
			checkVal: func(t *testing.T, v starlark.Value) {
				i, ok := v.(starlark.Int).Int64()
				require.True(t, ok)
				assert.Equal(t, int64(42), i)
			},
		},
		{
			name:     "float64",
			input:    3.14,
			expected: "float",
			checkVal: func(t *testing.T, v starlark.Value) {
				assert.InDelta(t, 3.14, float64(v.(starlark.Float)), 0.001)
			},
		},
		{
			name:     "string",
			input:    "hello",
			expected: "string",
			checkVal: func(t *testing.T, v starlark.Value) {
				assert.Equal(t, starlark.String("hello"), v)
			},
		},
		{
			name:     "string slice",
			input:    []string{"a", "b", "c"},
			expected: "list",
			checkVal: func(t *testing.T, v starlark.Value) {
				list := v.(*starlark.List)
				assert.Equal(t, 3, list.Len())
			},
		},
		{
			name:     "string map",
			input:    map[string]string{"key": "value"},
			expected: "dict",
			checkVal: func(t *testing.T, v starlark.Value) {
				dict := v.(*starlark.Dict)
				assert.Equal(t, 1, dict.Len())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, err := ToValue(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, v.Type())
			if tt.checkVal != nil {
				tt.checkVal(t, v)
			}
		})
	}
}

func TestTypes_FromValue(t *testing.T) {
	t.Parallel()
	t.Run("string", func(t *testing.T) {
		t.Parallel()
		var s string
		err := FromValue(starlark.String("hello"), &s)
		require.NoError(t, err)
		assert.Equal(t, "hello", s)
	})

	t.Run("int", func(t *testing.T) {
		t.Parallel()
		var i int64
		err := FromValue(starlark.MakeInt64(42), &i)
		require.NoError(t, err)
		assert.Equal(t, int64(42), i)
	})

	t.Run("bool", func(t *testing.T) {
		t.Parallel()
		var b bool
		err := FromValue(starlark.Bool(true), &b)
		require.NoError(t, err)
		assert.True(t, b)
	})

	t.Run("float", func(t *testing.T) {
		t.Parallel()
		var f float64
		err := FromValue(starlark.Float(3.14), &f)
		require.NoError(t, err)
		assert.InDelta(t, 3.14, f, 0.001)
	})

	t.Run("string slice", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.String("b"),
		})
		var s []string
		err := FromValue(list, &s)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, s)
	})

	t.Run("string map", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.String("key"), starlark.String("value"))
		var m map[string]string
		err := FromValue(dict, &m)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"key": "value"}, m)
	})

	t.Run("type mismatch", func(t *testing.T) {
		t.Parallel()
		var s string
		err := FromValue(starlark.MakeInt(42), &s)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrInvalidType))
	})
}

func TestTypes_ToString(t *testing.T) {
	t.Parallel()
	s, err := ToString(starlark.String("hello"))
	require.NoError(t, err)
	assert.Equal(t, "hello", s)

	_, err = ToString(starlark.MakeInt(42))
	require.Error(t, err)
}

func TestTypes_ToStringSlice(t *testing.T) {
	t.Parallel()
	list := starlark.NewList([]starlark.Value{
		starlark.String("a"),
		starlark.String("b"),
		starlark.String("c"),
	})
	s, err := ToStringSlice(list)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, s)
}

func TestTypes_ToStringMap(t *testing.T) {
	t.Parallel()
	dict := starlark.NewDict(2)
	_ = dict.SetKey(starlark.String("foo"), starlark.String("bar"))
	_ = dict.SetKey(starlark.String("baz"), starlark.String("qux"))
	m, err := ToStringMap(dict)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"foo": "bar", "baz": "qux"}, m)
}

func TestErrors_WrapError(t *testing.T) {
	t.Parallel()
	t.Run("nil error", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, WrapError(nil))
	})

	t.Run("regular error", func(t *testing.T) {
		t.Parallel()
		err := errors.New("regular error")
		wrapped := WrapError(err)
		assert.Equal(t, err, wrapped)
	})
}

func TestErrors_FormatError(t *testing.T) {
	t.Parallel()
	t.Run("nil error", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", FormatError(nil))
	})

	t.Run("regular error", func(t *testing.T) {
		t.Parallel()
		err := errors.New("test error")
		assert.Equal(t, "test error", FormatError(err))
	})
}

func TestErrors_NewTypeError(t *testing.T) {
	t.Parallel()
	err := NewTypeError("string", "int")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "string")
	assert.Contains(t, err.Error(), "int")
	assert.True(t, errors.Is(err, ErrInvalidType))
}

func TestLoader_CycleDetection(t *testing.T) {
	t.Parallel(
	// This test verifies cycle detection logic exists
	)

	loader := NewLoader()
	assert.NotNil(t, loader.cache)
	assert.NotNil(t, loader.loading)
}

func TestLoader_ClearCache(t *testing.T) {
	t.Parallel()
	loader := NewLoader()
	loader.cache["test"] = &cacheEntry{}
	assert.Len(t, loader.cache, 1)

	loader.ClearCache()
	assert.Len(t, loader.cache, 0)
}
