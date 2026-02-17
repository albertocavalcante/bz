package convertutil

import (
	"math"
	"testing"

	"go.starlark.net/starlark"
)

func TestToGoValue_None(t *testing.T) {
	t.Parallel()
	got, err := ToGoValue(starlark.None, ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestToGoValue_Bool(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   starlark.Bool
		want bool
	}{
		{starlark.True, true},
		{starlark.False, false},
	} {
		got, err := ToGoValue(tc.in, ToGoOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("ToGoValue(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestToGoValue_String(t *testing.T) {
	t.Parallel()
	got, err := ToGoValue(starlark.String("hello"), ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("expected %q, got %v", "hello", got)
	}
}

func TestToGoValue_Int(t *testing.T) {
	t.Parallel()
	got, err := ToGoValue(starlark.MakeInt(42), ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("expected 42, got %v", got)
	}
}

func TestToGoValue_IntNegative(t *testing.T) {
	t.Parallel()
	got, err := ToGoValue(starlark.MakeInt(-7), ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(-7) {
		t.Fatalf("expected -7, got %v", got)
	}
}

func TestToGoValue_IntOverflow_Error(t *testing.T) {
	t.Parallel()
	// Create a number larger than int64 max
	big := starlark.MakeUint64(math.MaxUint64)
	_, err := ToGoValue(big, ToGoOptions{})
	if err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestToGoValue_IntOverflow_AsString(t *testing.T) {
	t.Parallel()
	big := starlark.MakeUint64(math.MaxUint64)
	got, err := ToGoValue(big, ToGoOptions{IntOverflowAsString: true})
	if err != nil {
		t.Fatal(err)
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T", got)
	}
	if s == "" {
		t.Fatal("expected non-empty string representation")
	}
}

func TestToGoValue_Float(t *testing.T) {
	t.Parallel()
	got, err := ToGoValue(starlark.Float(3.14), ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", got)
	}
	if f != 3.14 {
		t.Fatalf("expected 3.14, got %v", f)
	}
}

func TestToGoValue_List(t *testing.T) {
	t.Parallel()
	list := starlark.NewList([]starlark.Value{
		starlark.String("a"),
		starlark.MakeInt(1),
		starlark.True,
	})
	got, err := ToGoValue(list, ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	slice, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(slice) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(slice))
	}
	if slice[0] != "a" {
		t.Fatalf("expected %q at [0], got %v", "a", slice[0])
	}
	if slice[1] != int64(1) {
		t.Fatalf("expected 1 at [1], got %v", slice[1])
	}
	if slice[2] != true {
		t.Fatalf("expected true at [2], got %v", slice[2])
	}
}

func TestToGoValue_Tuple(t *testing.T) {
	t.Parallel()
	tuple := starlark.Tuple{starlark.String("x"), starlark.String("y")}
	got, err := ToGoValue(tuple, ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	slice, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(slice) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(slice))
	}
}

func TestToGoValue_Dict(t *testing.T) {
	t.Parallel()
	d := starlark.NewDict(2)
	_ = d.SetKey(starlark.String("name"), starlark.String("rules_go"))
	_ = d.SetKey(starlark.String("version"), starlark.MakeInt(3))

	got, err := ToGoValue(d, ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}
	if m["name"] != "rules_go" {
		t.Fatalf("expected %q, got %v", "rules_go", m["name"])
	}
	if m["version"] != int64(3) {
		t.Fatalf("expected 3, got %v", m["version"])
	}
}

func TestToGoValue_Dict_NonStringKey(t *testing.T) {
	t.Parallel()
	d := starlark.NewDict(1)
	_ = d.SetKey(starlark.MakeInt(1), starlark.String("val"))

	_, err := ToGoValue(d, ToGoOptions{})
	if err == nil {
		t.Fatal("expected error for non-string dict key")
	}
}

func TestToGoValue_UnknownType_Error(t *testing.T) {
	t.Parallel()
	// A Builtin is a type not handled in the switch
	val := starlark.NewBuiltin("noop", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})
	_, err := ToGoValue(val, ToGoOptions{})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestToGoValue_UnknownType_AsValue(t *testing.T) {
	t.Parallel()
	val := starlark.NewBuiltin("noop", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})
	got, err := ToGoValue(val, ToGoOptions{UnknownAsValue: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != val {
		t.Fatalf("expected original value back, got %v", got)
	}
}

func TestToGoValue_EmptyList(t *testing.T) {
	t.Parallel()
	list := starlark.NewList(nil)
	got, err := ToGoValue(list, ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	slice, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(slice) != 0 {
		t.Fatalf("expected empty slice, got %d elements", len(slice))
	}
}

func TestToGoValue_EmptyDict(t *testing.T) {
	t.Parallel()
	d := starlark.NewDict(0)
	got, err := ToGoValue(d, ToGoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(m))
	}
}
