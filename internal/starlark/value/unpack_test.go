package value

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestUnpacker_Required(t *testing.T) {
	t.Run("unpacks required positional string", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("hello")}
		u := NewUnpacker("test_fn", args, nil)

		var s string
		if err := u.Required("name", &s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "hello" {
			t.Errorf("expected 'hello', got %q", s)
		}
	})

	t.Run("unpacks required kwarg string", func(t *testing.T) {
		kwargs := []starlark.Tuple{
			{starlark.String("name"), starlark.String("world")},
		}
		u := NewUnpacker("test_fn", nil, kwargs)

		var s string
		if err := u.Required("name", &s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "world" {
			t.Errorf("expected 'world', got %q", s)
		}
	})

	t.Run("returns error for missing required arg", func(t *testing.T) {
		u := NewUnpacker("test_fn", nil, nil)

		var s string
		err := u.Required("name", &s)
		if err == nil {
			t.Fatal("expected error for missing required arg")
		}
		if err.Error() != `test_fn: missing required argument "name"` {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("returns error for wrong type", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(42)}
		u := NewUnpacker("test_fn", args, nil)

		var s string
		err := u.Required("name", &s)
		if err == nil {
			t.Fatal("expected error for wrong type")
		}
	})

	t.Run("unpacks required int", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		u := NewUnpacker("test_fn", args, nil)

		var i int
		if err := u.Required("count", &i); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if i != 123 {
			t.Errorf("expected 123, got %d", i)
		}
	})

	t.Run("unpacks required bool", func(t *testing.T) {
		args := starlark.Tuple{starlark.True}
		u := NewUnpacker("test_fn", args, nil)

		var b bool
		if err := u.Required("flag", &b); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b {
			t.Error("expected true")
		}
	})

	t.Run("unpacks required string list", func(t *testing.T) {
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.String("b"),
			starlark.String("c"),
		})
		args := starlark.Tuple{list}
		u := NewUnpacker("test_fn", args, nil)

		var items []string
		if err := u.Required("items", &items); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 3 || items[0] != "a" || items[1] != "b" || items[2] != "c" {
			t.Errorf("unexpected items: %v", items)
		}
	})

	t.Run("unpacks required string dict", func(t *testing.T) {
		dict := starlark.NewDict(2)
		_ = dict.SetKey(starlark.String("key1"), starlark.String("value1"))
		_ = dict.SetKey(starlark.String("key2"), starlark.String("value2"))
		args := starlark.Tuple{dict}
		u := NewUnpacker("test_fn", args, nil)

		var m map[string]string
		if err := u.Required("data", &m); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["key1"] != "value1" || m["key2"] != "value2" {
			t.Errorf("unexpected map: %v", m)
		}
	})

	t.Run("unpacks starlark.Value", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("anything")}
		u := NewUnpacker("test_fn", args, nil)

		var v starlark.Value
		if err := u.Required("value", &v); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Type() != "string" {
			t.Errorf("expected string type, got %s", v.Type())
		}
	})
}

func TestUnpacker_Optional(t *testing.T) {
	t.Run("unpacks optional positional arg", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("provided")}
		u := NewUnpacker("test_fn", args, nil)

		var s string
		if err := u.Optional("name", &s, "default"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "provided" {
			t.Errorf("expected 'provided', got %q", s)
		}
	})

	t.Run("uses default when arg missing", func(t *testing.T) {
		u := NewUnpacker("test_fn", nil, nil)

		var s string
		if err := u.Optional("name", &s, "default_value"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "default_value" {
			t.Errorf("expected 'default_value', got %q", s)
		}
	})

	t.Run("uses nil default for string", func(t *testing.T) {
		u := NewUnpacker("test_fn", nil, nil)

		var s string
		if err := u.Optional("name", &s, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != "" {
			t.Errorf("expected empty string, got %q", s)
		}
	})

	t.Run("uses default int", func(t *testing.T) {
		u := NewUnpacker("test_fn", nil, nil)

		var i int
		if err := u.Optional("count", &i, 42); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if i != 42 {
			t.Errorf("expected 42, got %d", i)
		}
	})

	t.Run("uses default bool", func(t *testing.T) {
		u := NewUnpacker("test_fn", nil, nil)

		var b bool
		if err := u.Optional("flag", &b, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b {
			t.Error("expected true")
		}
	})

	t.Run("unpacks optional kwarg", func(t *testing.T) {
		kwargs := []starlark.Tuple{
			{starlark.String("timeout"), starlark.MakeInt(30)},
		}
		u := NewUnpacker("test_fn", nil, kwargs)

		var timeout int
		if err := u.Optional("timeout", &timeout, 10); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if timeout != 30 {
			t.Errorf("expected 30, got %d", timeout)
		}
	})
}

func TestUnpacker_Validate(t *testing.T) {
	t.Run("passes validation with no extra args", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("a"), starlark.String("b")}
		u := NewUnpacker("test_fn", args, nil)

		var a, b string
		_ = u.Required("a", &a)
		_ = u.Required("b", &b)

		if err := u.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails with unused positional args", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("a"), starlark.String("extra")}
		u := NewUnpacker("test_fn", args, nil)

		var a string
		_ = u.Required("a", &a)

		err := u.Validate()
		if err == nil {
			t.Fatal("expected error for unused positional arg")
		}
	})

	t.Run("fails with unknown kwargs", func(t *testing.T) {
		kwargs := []starlark.Tuple{
			{starlark.String("known"), starlark.String("value")},
			{starlark.String("unknown"), starlark.String("value")},
		}
		u := NewUnpacker("test_fn", nil, kwargs)

		var known string
		_ = u.Required("known", &known)

		err := u.Validate()
		if err == nil {
			t.Fatal("expected error for unknown kwarg")
		}
		if err.Error() != `test_fn: unexpected keyword argument "unknown"` {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestUnpacker_RemainingArgs(t *testing.T) {
	t.Run("returns remaining positional args", func(t *testing.T) {
		args := starlark.Tuple{
			starlark.String("first"),
			starlark.String("second"),
			starlark.String("third"),
		}
		u := NewUnpacker("test_fn", args, nil)

		var first string
		_ = u.Required("first", &first)

		remaining := u.RemainingArgs()
		if len(remaining) != 2 {
			t.Fatalf("expected 2 remaining args, got %d", len(remaining))
		}
	})

	t.Run("returns nil when no remaining args", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("only")}
		u := NewUnpacker("test_fn", args, nil)

		var only string
		_ = u.Required("only", &only)

		remaining := u.RemainingArgs()
		if remaining != nil {
			t.Errorf("expected nil, got %v", remaining)
		}
	})
}

func TestUnpacker_MixedArgs(t *testing.T) {
	t.Run("handles positional and keyword args together", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("positional")}
		kwargs := []starlark.Tuple{
			{starlark.String("kwarg"), starlark.MakeInt(42)},
		}
		u := NewUnpacker("test_fn", args, kwargs)

		var pos string
		var kw int
		if err := u.Required("pos", &pos); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := u.Required("kwarg", &kw); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := u.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pos != "positional" {
			t.Errorf("expected 'positional', got %q", pos)
		}
		if kw != 42 {
			t.Errorf("expected 42, got %d", kw)
		}
	})
}
