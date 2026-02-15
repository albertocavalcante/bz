package value

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestBase(t *testing.T) {
	t.Parallel()
	t.Run("NewBase creates base with name", func(t *testing.T) {
		t.Parallel()
		b := NewBase("test_type")
		if b.Name() != "test_type" {
			t.Errorf("expected name 'test_type', got %q", b.Name())
		}
	})

	t.Run("String returns name", func(t *testing.T) {
		t.Parallel()
		b := NewBase("my_value")
		if b.String() != "my_value" {
			t.Errorf("expected String() = 'my_value', got %q", b.String())
		}
	})

	t.Run("Type returns name", func(t *testing.T) {
		t.Parallel()
		b := NewBase("custom_type")
		if b.Type() != "custom_type" {
			t.Errorf("expected Type() = 'custom_type', got %q", b.Type())
		}
	})

	t.Run("Truth returns true", func(t *testing.T) {
		t.Parallel()
		b := NewBase("test")
		if b.Truth() != starlark.True {
			t.Error("expected Truth() = true")
		}
	})

	t.Run("Hash returns error", func(t *testing.T) {
		t.Parallel()
		b := NewBase("unhashable")
		_, err := b.Hash()
		if err == nil {
			t.Error("expected Hash() to return error")
		}
		if err.Error() != "unhashable type: unhashable" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Freeze does not panic", func(t *testing.T) {
		t.Parallel()
		b := NewBase("test")
		b.Freeze() // Should not panic
	})
}

func TestBaseImplementsValue(t *testing.T) {
	t.Parallel()
	var _ starlark.Value = (*Base)(nil)
}

func TestAttrAccessor(t *testing.T) {
	t.Parallel()
	t.Run("NewAttrAccessor creates empty accessor", func(t *testing.T) {
		t.Parallel()
		a := NewAttrAccessor()
		if a.Len() != 0 {
			t.Errorf("expected Len() = 0, got %d", a.Len())
		}
	})

	t.Run("Set and Get attribute", func(t *testing.T) {
		t.Parallel()
		a := NewAttrAccessor()
		a.Set("name", starlark.String("test"))

		v := a.Get("name")
		if v == nil {
			t.Fatal("expected to get value")
		}
		s, ok := v.(starlark.String)
		if !ok {
			t.Fatalf("expected string, got %T", v)
		}
		if string(s) != "test" {
			t.Errorf("expected 'test', got %q", s)
		}
	})

	t.Run("Attr returns value", func(t *testing.T) {
		t.Parallel()
		a := NewAttrAccessor()
		a.Set("count", starlark.MakeInt(42))

		v, err := a.Attr("count")
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

	t.Run("Attr returns error for missing attribute", func(t *testing.T) {
		t.Parallel()
		a := NewAttrAccessor()
		_, err := a.Attr("missing")
		if err == nil {
			t.Error("expected error for missing attribute")
		}
	})

	t.Run("AttrNames returns sorted names", func(t *testing.T) {
		t.Parallel()
		a := NewAttrAccessor()
		a.Set("zebra", starlark.String("z"))
		a.Set("alpha", starlark.String("a"))
		a.Set("beta", starlark.String("b"))

		names := a.AttrNames()
		if len(names) != 3 {
			t.Fatalf("expected 3 names, got %d", len(names))
		}
		if names[0] != "alpha" || names[1] != "beta" || names[2] != "zebra" {
			t.Errorf("names not sorted: %v", names)
		}
	})

	t.Run("Has returns true for existing attribute", func(t *testing.T) {
		t.Parallel()
		a := NewAttrAccessor()
		a.Set("exists", starlark.True)

		if !a.Has("exists") {
			t.Error("expected Has('exists') = true")
		}
		if a.Has("missing") {
			t.Error("expected Has('missing') = false")
		}
	})

	t.Run("Delete removes attribute", func(t *testing.T) {
		t.Parallel()
		a := NewAttrAccessor()
		a.Set("temp", starlark.String("value"))
		a.Delete("temp")

		if a.Has("temp") {
			t.Error("expected attribute to be deleted")
		}
	})

	t.Run("Clear removes all attributes", func(t *testing.T) {
		t.Parallel()
		a := NewAttrAccessor()
		a.Set("a", starlark.String("1"))
		a.Set("b", starlark.String("2"))
		a.Clear()

		if a.Len() != 0 {
			t.Errorf("expected Len() = 0 after Clear(), got %d", a.Len())
		}
	})

	t.Run("Get returns nil for missing attribute", func(t *testing.T) {
		t.Parallel()
		a := NewAttrAccessor()
		if a.Get("missing") != nil {
			t.Error("expected Get('missing') = nil")
		}
	})
}

// TestAttrAccessorProvidesHasAttrsMethods verifies that AttrAccessor
// provides the methods needed for HasAttrs when composed with Base.
func TestAttrAccessorProvidesHasAttrsMethods(t *testing.T) {
	t.Parallel(
	// AttrAccessor provides Attr and AttrNames, which are part of HasAttrs.
	// It's designed to be composed with Base to satisfy the full interface.
	)

	a := NewAttrAccessor()

	// Verify Attr method exists and works
	a.Set("test", starlark.String("value"))
	if _, err := a.Attr("test"); err != nil {
		t.Errorf("Attr method should work: %v", err)
	}

	// Verify AttrNames method exists and works
	names := a.AttrNames()
	if len(names) != 1 || names[0] != "test" {
		t.Errorf("AttrNames should return [\"test\"], got %v", names)
	}
}
