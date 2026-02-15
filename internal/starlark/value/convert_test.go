package value

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestToStarlark(t *testing.T) {
	t.Parallel()
	t.Run("converts nil to None", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != starlark.None {
			t.Errorf("expected None, got %v", v)
		}
	})

	t.Run("converts string", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark("hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s, ok := v.(starlark.String)
		if !ok {
			t.Fatalf("expected String, got %T", v)
		}
		if string(s) != "hello" {
			t.Errorf("expected 'hello', got %q", s)
		}
	})

	t.Run("converts bool", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark(true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != starlark.True {
			t.Errorf("expected True, got %v", v)
		}
	})

	t.Run("converts int", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark(42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		i, ok := v.(starlark.Int)
		if !ok {
			t.Fatalf("expected Int, got %T", v)
		}
		val, _ := i.Int64()
		if val != 42 {
			t.Errorf("expected 42, got %d", val)
		}
	})

	t.Run("converts int64", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark(int64(9999999999))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		i, ok := v.(starlark.Int)
		if !ok {
			t.Fatalf("expected Int, got %T", v)
		}
		val, _ := i.Int64()
		if val != 9999999999 {
			t.Errorf("expected 9999999999, got %d", val)
		}
	})

	t.Run("converts float64", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark(3.14)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		f, ok := v.(starlark.Float)
		if !ok {
			t.Fatalf("expected Float, got %T", v)
		}
		if float64(f) != 3.14 {
			t.Errorf("expected 3.14, got %f", f)
		}
	})

	t.Run("converts string slice", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark([]string{"a", "b", "c"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		list, ok := v.(*starlark.List)
		if !ok {
			t.Fatalf("expected List, got %T", v)
		}
		if list.Len() != 3 {
			t.Errorf("expected length 3, got %d", list.Len())
		}
	})

	t.Run("converts int slice", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark([]int{1, 2, 3})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		list, ok := v.(*starlark.List)
		if !ok {
			t.Fatalf("expected List, got %T", v)
		}
		if list.Len() != 3 {
			t.Errorf("expected length 3, got %d", list.Len())
		}
	})

	t.Run("converts string map", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark(map[string]string{"key": "value"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		dict, ok := v.(*starlark.Dict)
		if !ok {
			t.Fatalf("expected Dict, got %T", v)
		}
		if dict.Len() != 1 {
			t.Errorf("expected length 1, got %d", dict.Len())
		}
	})

	t.Run("converts nested structure", func(t *testing.T) {
		t.Parallel()
		v, err := ToStarlark(map[string]any{
			"strings": []string{"a", "b"},
			"number":  42,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		dict, ok := v.(*starlark.Dict)
		if !ok {
			t.Fatalf("expected Dict, got %T", v)
		}
		if dict.Len() != 2 {
			t.Errorf("expected length 2, got %d", dict.Len())
		}
	})

	t.Run("passes through starlark values", func(t *testing.T) {
		t.Parallel()
		original := starlark.String("already starlark")
		v, err := ToStarlark(original)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != original {
			t.Error("expected same value")
		}
	})

	t.Run("converts nil pointer to None", func(t *testing.T) {
		t.Parallel()
		var ptr *string
		v, err := ToStarlark(ptr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != starlark.None {
			t.Errorf("expected None, got %v", v)
		}
	})

	t.Run("errors on unsupported type", func(t *testing.T) {
		t.Parallel()
		_, err := ToStarlark(make(chan int))
		if err == nil {
			t.Error("expected error for unsupported type")
		}
	})
}

func TestFromStarlark(t *testing.T) {
	t.Parallel()
	t.Run("converts to string", func(t *testing.T) {
		t.Parallel()
		var s string
		err := FromStarlark(starlark.String("hello"), &s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "hello" {
			t.Errorf("expected 'hello', got %q", s)
		}
	})

	t.Run("converts to bool", func(t *testing.T) {
		t.Parallel()
		var b bool
		err := FromStarlark(starlark.True, &b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b {
			t.Error("expected true")
		}
	})

	t.Run("converts to int", func(t *testing.T) {
		t.Parallel()
		var i int
		err := FromStarlark(starlark.MakeInt(42), &i)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if i != 42 {
			t.Errorf("expected 42, got %d", i)
		}
	})

	t.Run("converts to int64", func(t *testing.T) {
		t.Parallel()
		var i int64
		err := FromStarlark(starlark.MakeInt64(9999999999), &i)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if i != 9999999999 {
			t.Errorf("expected 9999999999, got %d", i)
		}
	})

	t.Run("converts to float64 from Float", func(t *testing.T) {
		t.Parallel()
		var f float64
		err := FromStarlark(starlark.Float(3.14), &f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f != 3.14 {
			t.Errorf("expected 3.14, got %f", f)
		}
	})

	t.Run("converts to float64 from Int", func(t *testing.T) {
		t.Parallel()
		var f float64
		err := FromStarlark(starlark.MakeInt(42), &f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f != 42.0 {
			t.Errorf("expected 42.0, got %f", f)
		}
	})

	t.Run("converts to string slice", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.String("b"),
		})
		var s []string
		err := FromStarlark(list, &s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(s) != 2 || s[0] != "a" || s[1] != "b" {
			t.Errorf("unexpected slice: %v", s)
		}
	})

	t.Run("converts to interface slice", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.MakeInt(42),
		})
		var s []any
		err := FromStarlark(list, &s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(s) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(s))
		}
		if s[0] != "a" {
			t.Errorf("expected first element 'a', got %v", s[0])
		}
		if s[1] != int64(42) {
			t.Errorf("expected second element 42, got %v", s[1])
		}
	})

	t.Run("converts to string map", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.String("key"), starlark.String("value"))

		var m map[string]string
		err := FromStarlark(dict, &m)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["key"] != "value" {
			t.Errorf("expected m['key'] = 'value', got %q", m["key"])
		}
	})

	t.Run("converts to interface map", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(2)
		_ = dict.SetKey(starlark.String("str"), starlark.String("value"))
		_ = dict.SetKey(starlark.String("num"), starlark.MakeInt(42))

		var m map[string]any
		err := FromStarlark(dict, &m)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["str"] != "value" {
			t.Errorf("expected m['str'] = 'value', got %v", m["str"])
		}
		if m["num"] != int64(42) {
			t.Errorf("expected m['num'] = 42, got %v", m["num"])
		}
	})

	t.Run("converts to starlark.Value", func(t *testing.T) {
		t.Parallel()
		original := starlark.String("test")
		var v starlark.Value
		err := FromStarlark(original, &v)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != original {
			t.Error("expected same value")
		}
	})

	t.Run("errors on type mismatch", func(t *testing.T) {
		t.Parallel()
		var s string
		err := FromStarlark(starlark.MakeInt(42), &s)
		if err == nil {
			t.Error("expected error for type mismatch")
		}
	})

	t.Run("errors on nil destination", func(t *testing.T) {
		t.Parallel()
		err := FromStarlark(starlark.String("test"), nil)
		if err == nil {
			t.Error("expected error for nil destination")
		}
	})

	t.Run("errors on non-pointer destination", func(t *testing.T) {
		t.Parallel()
		var s string
		err := FromStarlark(starlark.String("test"), s)
		if err == nil {
			t.Error("expected error for non-pointer destination")
		}
	})
}

func TestMustString(t *testing.T) {
	t.Parallel()
	t.Run("extracts string", func(t *testing.T) {
		t.Parallel()
		s, err := MustString(starlark.String("hello"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "hello" {
			t.Errorf("expected 'hello', got %q", s)
		}
	})

	t.Run("errors on non-string", func(t *testing.T) {
		t.Parallel()
		_, err := MustString(starlark.MakeInt(42))
		if err == nil {
			t.Error("expected error for non-string")
		}
	})
}

func TestMustStringList(t *testing.T) {
	t.Parallel()
	t.Run("extracts string list", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.String("b"),
		})
		s, err := MustStringList(list)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(s) != 2 || s[0] != "a" || s[1] != "b" {
			t.Errorf("unexpected list: %v", s)
		}
	})

	t.Run("errors on non-list", func(t *testing.T) {
		t.Parallel()
		_, err := MustStringList(starlark.String("not a list"))
		if err == nil {
			t.Error("expected error for non-list")
		}
	})

	t.Run("errors on non-string element", func(t *testing.T) {
		t.Parallel()
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.MakeInt(42),
		})
		_, err := MustStringList(list)
		if err == nil {
			t.Error("expected error for non-string element")
		}
	})
}

func TestMustStringDict(t *testing.T) {
	t.Parallel()
	t.Run("extracts string dict", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.String("key"), starlark.String("value"))

		m, err := MustStringDict(dict)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["key"] != "value" {
			t.Errorf("expected m['key'] = 'value', got %q", m["key"])
		}
	})

	t.Run("errors on non-dict", func(t *testing.T) {
		t.Parallel()
		_, err := MustStringDict(starlark.String("not a dict"))
		if err == nil {
			t.Error("expected error for non-dict")
		}
	})

	t.Run("errors on non-string key", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.MakeInt(42), starlark.String("value"))

		_, err := MustStringDict(dict)
		if err == nil {
			t.Error("expected error for non-string key")
		}
	})

	t.Run("errors on non-string value", func(t *testing.T) {
		t.Parallel()
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.String("key"), starlark.MakeInt(42))

		_, err := MustStringDict(dict)
		if err == nil {
			t.Error("expected error for non-string value")
		}
	})
}

func TestMustBool(t *testing.T) {
	t.Parallel()
	t.Run("extracts true", func(t *testing.T) {
		t.Parallel()
		b, err := MustBool(starlark.True)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b {
			t.Error("expected true")
		}
	})

	t.Run("extracts false", func(t *testing.T) {
		t.Parallel()
		b, err := MustBool(starlark.False)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b {
			t.Error("expected false")
		}
	})

	t.Run("errors on non-bool", func(t *testing.T) {
		t.Parallel()
		_, err := MustBool(starlark.String("true"))
		if err == nil {
			t.Error("expected error for non-bool")
		}
	})
}

func TestMustInt(t *testing.T) {
	t.Parallel()
	t.Run("extracts int", func(t *testing.T) {
		t.Parallel()
		i, err := MustInt(starlark.MakeInt(42))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if i != 42 {
			t.Errorf("expected 42, got %d", i)
		}
	})

	t.Run("errors on non-int", func(t *testing.T) {
		t.Parallel()
		_, err := MustInt(starlark.String("42"))
		if err == nil {
			t.Error("expected error for non-int")
		}
	})
}

func TestMustFloat(t *testing.T) {
	t.Parallel()
	t.Run("extracts float", func(t *testing.T) {
		t.Parallel()
		f, err := MustFloat(starlark.Float(3.14))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f != 3.14 {
			t.Errorf("expected 3.14, got %f", f)
		}
	})

	t.Run("extracts float from int", func(t *testing.T) {
		t.Parallel()
		f, err := MustFloat(starlark.MakeInt(42))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f != 42.0 {
			t.Errorf("expected 42.0, got %f", f)
		}
	})

	t.Run("errors on non-numeric", func(t *testing.T) {
		t.Parallel()
		_, err := MustFloat(starlark.String("3.14"))
		if err == nil {
			t.Error("expected error for non-numeric")
		}
	})
}

func TestNewStruct(t *testing.T) {
	t.Parallel()
	t.Run("creates struct with attributes", func(t *testing.T) {
		t.Parallel()
		s := NewStruct(map[string]starlark.Value{
			"name":  starlark.String("test"),
			"count": starlark.MakeInt(42),
		})

		v, err := s.Attr("name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(v.(starlark.String)) != "test" {
			t.Errorf("expected 'test', got %v", v)
		}
	})
}

func TestNewNamedStruct(t *testing.T) {
	t.Parallel()
	t.Run("creates named struct", func(t *testing.T) {
		t.Parallel()
		s := NewNamedStruct("my_type", map[string]starlark.Value{
			"data": starlark.String("value"),
		})

		// The constructor name appears in the String() representation
		str := s.String()
		if str != `my_type(data = "value")` {
			t.Errorf("expected String() to include constructor name, got %q", str)
		}

		// Note: Type() always returns "struct" for starlarkstruct.Struct
		// The constructor name is only used in String() representation
		if s.Type() != "struct" {
			t.Errorf("expected type 'struct', got %q", s.Type())
		}
	})
}
