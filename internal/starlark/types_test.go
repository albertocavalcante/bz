package starlark

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

// --- ToValue ---

func TestToValue_Nil(t *testing.T) {
	t.Parallel()
	v, err := ToValue(nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, v)
}

func TestToValue_StarlarkValuePassthrough(t *testing.T) {
	t.Parallel()
	original := starlark.String("already starlark")
	v, err := ToValue(original)
	require.NoError(t, err)
	assert.Equal(t, original, v)
}

func TestToValue_BoolValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    bool
		expected starlark.Bool
	}{
		{"true", true, starlark.True},
		{"false", false, starlark.False},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, err := ToValue(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, v)
		})
	}
}

func TestToValue_IntegerTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected int64
	}{
		{"int", int(42), 42},
		{"int8", int8(127), 127},
		{"int16", int16(32767), 32767},
		{"int32", int32(2147483647), 2147483647},
		{"int64", int64(9223372036854775807), 9223372036854775807},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, err := ToValue(tt.input)
			require.NoError(t, err)
			i, ok := v.(starlark.Int)
			require.True(t, ok, "expected Int, got %T", v)
			val, ok := i.Int64()
			require.True(t, ok)
			assert.Equal(t, tt.expected, val)
		})
	}
}

func TestToValue_UnsignedIntegerTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected uint64
	}{
		{"uint", uint(42), 42},
		{"uint8", uint8(255), 255},
		{"uint16", uint16(65535), 65535},
		{"uint32", uint32(4294967295), 4294967295},
		{"uint64", uint64(18446744073709551615), 18446744073709551615},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, err := ToValue(tt.input)
			require.NoError(t, err)
			i, ok := v.(starlark.Int)
			require.True(t, ok, "expected Int, got %T", v)
			val, ok := i.Uint64()
			require.True(t, ok)
			assert.Equal(t, tt.expected, val)
		})
	}
}

func TestToValue_FloatTypes(t *testing.T) {
	t.Parallel()

	t.Run("float32", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue(float32(3.14))
		require.NoError(t, err)
		f, ok := v.(starlark.Float)
		require.True(t, ok)
		assert.InDelta(t, 3.14, float64(f), 0.01)
	})

	t.Run("float64", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue(float64(3.14159265))
		require.NoError(t, err)
		f, ok := v.(starlark.Float)
		require.True(t, ok)
		assert.InDelta(t, 3.14159265, float64(f), 0.0001)
	})

	t.Run("float64 zero", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue(float64(0.0))
		require.NoError(t, err)
		f, ok := v.(starlark.Float)
		require.True(t, ok)
		assert.Equal(t, float64(0), float64(f))
	})
}

func TestToValue_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"normal string", "hello"},
		{"empty string", ""},
		{"unicode string", "Hello"},
		{"string with newlines", "line1\nline2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, err := ToValue(tt.input)
			require.NoError(t, err)
			s, ok := v.(starlark.String)
			require.True(t, ok)
			assert.Equal(t, tt.input, string(s))
		})
	}
}

func TestToValue_SliceTypes(t *testing.T) {
	t.Parallel()

	t.Run("string slice", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue([]string{"a", "b", "c"})
		require.NoError(t, err)
		list, ok := v.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 3, list.Len())
		assert.Equal(t, starlark.String("a"), list.Index(0))
		assert.Equal(t, starlark.String("b"), list.Index(1))
		assert.Equal(t, starlark.String("c"), list.Index(2))
	})

	t.Run("int slice", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue([]int{1, 2, 3})
		require.NoError(t, err)
		list, ok := v.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 3, list.Len())
	})

	t.Run("empty slice", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue([]string{})
		require.NoError(t, err)
		list, ok := v.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 0, list.Len())
	})

	t.Run("nested slice", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue([][]string{{"a", "b"}, {"c"}})
		require.NoError(t, err)
		list, ok := v.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 2, list.Len())
		inner, ok := list.Index(0).(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 2, inner.Len())
	})

	t.Run("array", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue([3]int{10, 20, 30})
		require.NoError(t, err)
		list, ok := v.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 3, list.Len())
	})
}

func TestToValue_MapTypes(t *testing.T) {
	t.Parallel()

	t.Run("string-string map", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue(map[string]string{"key": "value"})
		require.NoError(t, err)
		dict, ok := v.(*starlark.Dict)
		require.True(t, ok)
		assert.Equal(t, 1, dict.Len())
		val, found, err := dict.Get(starlark.String("key"))
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, starlark.String("value"), val)
	})

	t.Run("string-int map", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue(map[string]int{"count": 42})
		require.NoError(t, err)
		dict, ok := v.(*starlark.Dict)
		require.True(t, ok)
		assert.Equal(t, 1, dict.Len())
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()
		v, err := ToValue(map[string]string{})
		require.NoError(t, err)
		dict, ok := v.(*starlark.Dict)
		require.True(t, ok)
		assert.Equal(t, 0, dict.Len())
	})
}

func TestToValue_PointerTypes(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer", func(t *testing.T) {
		t.Parallel()
		var p *string
		v, err := ToValue(p)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, v)
	})

	t.Run("non-nil pointer", func(t *testing.T) {
		t.Parallel()
		s := "hello"
		v, err := ToValue(&s)
		require.NoError(t, err)
		sv, ok := v.(starlark.String)
		require.True(t, ok)
		assert.Equal(t, "hello", string(sv))
	})
}

func TestToValue_InterfaceType(t *testing.T) {
	t.Parallel()

	t.Run("nil interface", func(t *testing.T) {
		t.Parallel()
		var iface any
		v, err := ToValue(iface)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, v)
	})
}

func TestToValue_UnsupportedType(t *testing.T) {
	t.Parallel()

	_, err := ToValue(make(chan int))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidType)
}

// --- FromValue ---

func TestFromValue_NilTarget(t *testing.T) {
	t.Parallel()
	err := FromValue(starlark.String("test"), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidType)
}

func TestFromValue_NonPointerTarget(t *testing.T) {
	t.Parallel()
	err := FromValue(starlark.String("test"), "not a pointer")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidType)
}

func TestFromValue_NilPointerTarget(t *testing.T) {
	t.Parallel()
	var sp *string
	err := FromValue(starlark.String("test"), sp)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidType)
}

func TestFromValue_NoneToZeroValue(t *testing.T) {
	t.Parallel()

	t.Run("none to string", func(t *testing.T) {
		t.Parallel()
		s := "original"
		err := FromValue(starlark.None, &s)
		require.NoError(t, err)
		assert.Equal(t, "", s)
	})

	t.Run("none to int", func(t *testing.T) {
		t.Parallel()
		i := int64(42)
		err := FromValue(starlark.None, &i)
		require.NoError(t, err)
		assert.Equal(t, int64(0), i)
	})

	t.Run("none to bool", func(t *testing.T) {
		t.Parallel()
		b := true
		err := FromValue(starlark.None, &b)
		require.NoError(t, err)
		assert.False(t, b)
	})

	t.Run("none to float", func(t *testing.T) {
		t.Parallel()
		f := float64(3.14)
		err := FromValue(starlark.None, &f)
		require.NoError(t, err)
		assert.Equal(t, float64(0), f)
	})
}

func TestFromValue_Bool(t *testing.T) {
	t.Parallel()

	t.Run("true", func(t *testing.T) {
		t.Parallel()
		var b bool
		require.NoError(t, FromValue(starlark.True, &b))
		assert.True(t, b)
	})

	t.Run("false", func(t *testing.T) {
		t.Parallel()
		var b bool
		require.NoError(t, FromValue(starlark.False, &b))
		assert.False(t, b)
	})

	t.Run("type mismatch", func(t *testing.T) {
		t.Parallel()
		var b bool
		err := FromValue(starlark.String("true"), &b)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidType)
	})
}

func TestFromValue_IntTypes(t *testing.T) {
	t.Parallel()

	t.Run("to int", func(t *testing.T) {
		t.Parallel()
		var i int
		require.NoError(t, FromValue(starlark.MakeInt(42), &i))
		assert.Equal(t, 42, i)
	})

	t.Run("to int8", func(t *testing.T) {
		t.Parallel()
		var i int8
		require.NoError(t, FromValue(starlark.MakeInt(127), &i))
		assert.Equal(t, int8(127), i)
	})

	t.Run("to int16", func(t *testing.T) {
		t.Parallel()
		var i int16
		require.NoError(t, FromValue(starlark.MakeInt(32767), &i))
		assert.Equal(t, int16(32767), i)
	})

	t.Run("to int32", func(t *testing.T) {
		t.Parallel()
		var i int32
		require.NoError(t, FromValue(starlark.MakeInt(100), &i))
		assert.Equal(t, int32(100), i)
	})

	t.Run("to int64", func(t *testing.T) {
		t.Parallel()
		var i int64
		require.NoError(t, FromValue(starlark.MakeInt64(9223372036854775807), &i))
		assert.Equal(t, int64(9223372036854775807), i)
	})

	t.Run("type mismatch int", func(t *testing.T) {
		t.Parallel()
		var i int64
		err := FromValue(starlark.String("42"), &i)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidType)
	})
}

func TestFromValue_UintTypes(t *testing.T) {
	t.Parallel()

	t.Run("to uint", func(t *testing.T) {
		t.Parallel()
		var u uint
		require.NoError(t, FromValue(starlark.MakeInt(42), &u))
		assert.Equal(t, uint(42), u)
	})

	t.Run("to uint64", func(t *testing.T) {
		t.Parallel()
		var u uint64
		require.NoError(t, FromValue(starlark.MakeUint64(18446744073709551615), &u))
		assert.Equal(t, uint64(18446744073709551615), u)
	})

	t.Run("type mismatch uint", func(t *testing.T) {
		t.Parallel()
		var u uint
		err := FromValue(starlark.String("42"), &u)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidType)
	})
}

func TestFromValue_Float(t *testing.T) {
	t.Parallel()

	t.Run("float32", func(t *testing.T) {
		t.Parallel()
		var f float32
		require.NoError(t, FromValue(starlark.Float(3.14), &f))
		assert.InDelta(t, float32(3.14), f, 0.01)
	})

	t.Run("float64", func(t *testing.T) {
		t.Parallel()
		var f float64
		require.NoError(t, FromValue(starlark.Float(3.14159), &f))
		assert.InDelta(t, 3.14159, f, 0.0001)
	})

	t.Run("int to float", func(t *testing.T) {
		t.Parallel()
		var f float64
		require.NoError(t, FromValue(starlark.MakeInt(42), &f))
		assert.Equal(t, float64(42), f)
	})

	t.Run("type mismatch float", func(t *testing.T) {
		t.Parallel()
		var f float64
		err := FromValue(starlark.String("3.14"), &f)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidType)
	})
}

func TestFromValue_String(t *testing.T) {
	t.Parallel()

	t.Run("normal string", func(t *testing.T) {
		t.Parallel()
		var s string
		require.NoError(t, FromValue(starlark.String("hello"), &s))
		assert.Equal(t, "hello", s)
	})

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()
		var s string
		require.NoError(t, FromValue(starlark.String(""), &s))
		assert.Equal(t, "", s)
	})

	t.Run("type mismatch string", func(t *testing.T) {
		t.Parallel()
		var s string
		err := FromValue(starlark.MakeInt(42), &s)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidType)
	})
}

func TestFromValue_ListToSlice(t *testing.T) {
	t.Parallel()

	t.Run("string list", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.String("b"),
			starlark.String("c"),
		})
		var s []string
		require.NoError(t, FromValue(list, &s))
		assert.Equal(t, []string{"a", "b", "c"}, s)
	})

	t.Run("int list", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList([]starlark.Value{
			starlark.MakeInt(1),
			starlark.MakeInt(2),
			starlark.MakeInt(3),
		})
		var s []int64
		require.NoError(t, FromValue(list, &s))
		assert.Equal(t, []int64{1, 2, 3}, s)
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList(nil)
		var s []string
		require.NoError(t, FromValue(list, &s))
		assert.Empty(t, s)
	})

	t.Run("tuple to slice", func(t *testing.T) {
		t.Parallel()
		tuple := starlark.Tuple{
			starlark.String("x"),
			starlark.String("y"),
		}
		var s []string
		require.NoError(t, FromValue(tuple, &s))
		assert.Equal(t, []string{"x", "y"}, s)
	})

	t.Run("wrong element type", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.MakeInt(42), // wrong type for []string target
		})
		var s []string
		err := FromValue(list, &s)
		require.Error(t, err)
	})

	t.Run("non-list to slice", func(t *testing.T) {
		t.Parallel()
		var s []string
		err := FromValue(starlark.String("not a list"), &s)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidType)
	})
}

func TestFromValue_DictToMap(t *testing.T) {
	t.Parallel()

	t.Run("string-string dict", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(2)
		_ = dict.SetKey(starlark.String("foo"), starlark.String("bar"))
		_ = dict.SetKey(starlark.String("baz"), starlark.String("qux"))
		var m map[string]string
		require.NoError(t, FromValue(dict, &m))
		assert.Equal(t, map[string]string{"foo": "bar", "baz": "qux"}, m)
	})

	t.Run("string-int dict", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.String("count"), starlark.MakeInt(42))
		var m map[string]int64
		require.NoError(t, FromValue(dict, &m))
		assert.Equal(t, map[string]int64{"count": 42}, m)
	})

	t.Run("empty dict", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(0)
		var m map[string]string
		require.NoError(t, FromValue(dict, &m))
		assert.Empty(t, m)
	})

	t.Run("non-dict to map", func(t *testing.T) {
		t.Parallel()
		var m map[string]string
		err := FromValue(starlark.String("not a dict"), &m)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidType)
	})
}

func TestFromValue_Interface(t *testing.T) {
	t.Parallel()

	t.Run("string to interface", func(t *testing.T) {
		t.Parallel()
		var v any
		require.NoError(t, FromValue(starlark.String("hello"), &v))
		assert.Equal(t, "hello", v)
	})

	t.Run("int to interface", func(t *testing.T) {
		t.Parallel()
		var v any
		require.NoError(t, FromValue(starlark.MakeInt64(42), &v))
		assert.Equal(t, int64(42), v)
	})

	t.Run("bool to interface", func(t *testing.T) {
		t.Parallel()
		var v any
		require.NoError(t, FromValue(starlark.True, &v))
		assert.Equal(t, true, v)
	})

	t.Run("none to interface", func(t *testing.T) {
		t.Parallel()
		var v any = "was something"
		require.NoError(t, FromValue(starlark.None, &v))
		assert.Nil(t, v)
	})

	t.Run("list to interface", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList([]starlark.Value{starlark.String("a"), starlark.MakeInt(1)})
		var v any
		require.NoError(t, FromValue(list, &v))
		slice, ok := v.([]any)
		require.True(t, ok)
		assert.Equal(t, "a", slice[0])
		assert.Equal(t, int64(1), slice[1])
	})

	t.Run("dict to interface", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.String("key"), starlark.String("val"))
		var v any
		require.NoError(t, FromValue(dict, &v))
		m, ok := v.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "val", m["key"])
	})
}

func TestFromValue_UnsupportedTargetType(t *testing.T) {
	t.Parallel()

	type custom struct{ X int }
	var c custom
	err := FromValue(starlark.String("test"), &c)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidType)
}

// --- toGoValue ---

func TestToGoValue_AllTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    starlark.Value
		expected any
	}{
		{"none", starlark.None, nil},
		{"true", starlark.True, true},
		{"false", starlark.False, false},
		{"small int", starlark.MakeInt(42), int64(42)},
		{"float", starlark.Float(3.14), float64(3.14)},
		{"string", starlark.String("hello"), "hello"},
		{"empty string", starlark.String(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Use the interface target path in FromValue
			var result any
			err := FromValue(tt.input, &result)
			if tt.input == starlark.None {
				// None sets zero value on interface, which is nil
				require.NoError(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToGoValue_NonStringDictKey(t *testing.T) {
	t.Parallel()

	// Create dict with non-string key
	dict := starlark.NewDict(1)
	_ = dict.SetKey(starlark.MakeInt(1), starlark.String("value"))

	var m map[string]string
	err := FromValue(dict, &m)
	require.Error(t, err)
}

// --- ToString ---

func TestToString_Valid(t *testing.T) {
	t.Parallel()
	s, err := ToString(starlark.String("hello"))
	require.NoError(t, err)
	assert.Equal(t, "hello", s)
}

func TestToString_Empty(t *testing.T) {
	t.Parallel()
	s, err := ToString(starlark.String(""))
	require.NoError(t, err)
	assert.Equal(t, "", s)
}

func TestToString_WrongType(t *testing.T) {
	t.Parallel()
	_, err := ToString(starlark.MakeInt(42))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidType)
}

// --- ToStringSlice ---

func TestToStringSlice_Valid(t *testing.T) {
	t.Parallel()
	list := starlark.NewList([]starlark.Value{
		starlark.String("a"),
		starlark.String("b"),
	})
	s, err := ToStringSlice(list)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, s)
}

func TestToStringSlice_EmptyList(t *testing.T) {
	t.Parallel()
	list := starlark.NewList(nil)
	s, err := ToStringSlice(list)
	require.NoError(t, err)
	assert.Empty(t, s)
}

func TestToStringSlice_WrongType(t *testing.T) {
	t.Parallel()
	_, err := ToStringSlice(starlark.String("not a list"))
	require.Error(t, err)
}

func TestToStringSlice_MixedTypes(t *testing.T) {
	t.Parallel()
	list := starlark.NewList([]starlark.Value{
		starlark.String("ok"),
		starlark.MakeInt(42),
	})
	_, err := ToStringSlice(list)
	require.Error(t, err)
}

// --- ToStringMap ---

func TestToStringMap_Valid(t *testing.T) {
	t.Parallel()
	dict := starlark.NewDict(2)
	_ = dict.SetKey(starlark.String("a"), starlark.String("1"))
	_ = dict.SetKey(starlark.String("b"), starlark.String("2"))
	m, err := ToStringMap(dict)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, m)
}

func TestToStringMap_EmptyDict(t *testing.T) {
	t.Parallel()
	dict := starlark.NewDict(0)
	m, err := ToStringMap(dict)
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestToStringMap_WrongType(t *testing.T) {
	t.Parallel()
	_, err := ToStringMap(starlark.String("not a dict"))
	require.Error(t, err)
}

func TestToStringMap_NonStringValue(t *testing.T) {
	t.Parallel()
	dict := starlark.NewDict(1)
	_ = dict.SetKey(starlark.String("key"), starlark.MakeInt(42))
	_, err := ToStringMap(dict)
	require.Error(t, err)
}

func TestToStringMap_NonStringKey(t *testing.T) {
	t.Parallel()
	dict := starlark.NewDict(1)
	_ = dict.SetKey(starlark.MakeInt(1), starlark.String("val"))
	_, err := ToStringMap(dict)
	require.Error(t, err)
}

// --- Round-trip tests ---

func TestRoundTrip_StringSlice(t *testing.T) {
	t.Parallel()

	original := []string{"alpha", "beta", "gamma"}
	v, err := ToValue(original)
	require.NoError(t, err)

	var result []string
	require.NoError(t, FromValue(v, &result))
	assert.Equal(t, original, result)
}

func TestRoundTrip_StringMap(t *testing.T) {
	t.Parallel()

	original := map[string]string{"name": "test", "version": "1.0"}
	v, err := ToValue(original)
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, FromValue(v, &result))
	assert.Equal(t, original, result)
}

func TestRoundTrip_Int(t *testing.T) {
	t.Parallel()

	original := int64(42)
	v, err := ToValue(original)
	require.NoError(t, err)

	var result int64
	require.NoError(t, FromValue(v, &result))
	assert.Equal(t, original, result)
}

func TestRoundTrip_Bool(t *testing.T) {
	t.Parallel()

	for _, original := range []bool{true, false} {
		v, err := ToValue(original)
		require.NoError(t, err)

		var result bool
		require.NoError(t, FromValue(v, &result))
		assert.Equal(t, original, result)
	}
}

// --- Error assertions ---

func TestFromValue_ErrorsAreErrInvalidType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  starlark.Value
		target any
	}{
		{"string to int", starlark.String("oops"), new(int64)},
		{"int to string", starlark.MakeInt(42), new(string)},
		{"bool to string", starlark.True, new(string)},
		{"string to bool", starlark.String("true"), new(bool)},
		{"string to float", starlark.String("3.14"), new(float64)},
		{"string to uint", starlark.String("42"), new(uint64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := FromValue(tt.value, tt.target)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidType), "expected ErrInvalidType, got: %v", err)
		})
	}
}
