package value

import (
	"errors"
	"testing"

	"go.starlark.net/starlark"
)

func TestWrapFunc(t *testing.T) {
	t.Parallel()
	t.Run("wraps simple function with required arg", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Name string `starlark:"name,required"`
		}

		fn := WrapFunc("greet", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.String("Hello, " + args.Name), nil
		})

		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, fn, starlark.Tuple{starlark.String("World")}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		s, ok := result.(starlark.String)
		if !ok {
			t.Fatalf("expected String, got %T", result)
		}
		if string(s) != "Hello, World" {
			t.Errorf("expected 'Hello, World', got %q", s)
		}
	})

	t.Run("handles optional args with defaults", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Name    string `starlark:"name,required"`
			Timeout int    `starlark:"timeout"`
		}

		fn := WrapFunc("fetch", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.MakeInt(args.Timeout), nil
		})

		thread := &starlark.Thread{Name: "test"}

		// Without optional arg - should use zero value
		result, err := starlark.Call(thread, fn, starlark.Tuple{starlark.String("url")}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		i, _ := result.(starlark.Int).Int64()
		if i != 0 {
			t.Errorf("expected 0 (default), got %d", i)
		}

		// With optional arg
		result, err = starlark.Call(thread, fn, starlark.Tuple{starlark.String("url"), starlark.MakeInt(30)}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		i, _ = result.(starlark.Int).Int64()
		if i != 30 {
			t.Errorf("expected 30, got %d", i)
		}
	})

	t.Run("handles kwargs", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			URL     string `starlark:"url,required"`
			Timeout int    `starlark:"timeout"`
		}

		fn := WrapFunc("fetch", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.MakeInt(args.Timeout), nil
		})

		thread := &starlark.Thread{Name: "test"}
		kwargs := []starlark.Tuple{
			{starlark.String("url"), starlark.String("http://example.com")},
			{starlark.String("timeout"), starlark.MakeInt(60)},
		}

		result, err := starlark.Call(thread, fn, nil, kwargs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		i, _ := result.(starlark.Int).Int64()
		if i != 60 {
			t.Errorf("expected 60, got %d", i)
		}
	})

	t.Run("handles bool arg", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Enabled bool `starlark:"enabled,required"`
		}

		fn := WrapFunc("toggle", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.Bool(args.Enabled), nil
		})

		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, fn, starlark.Tuple{starlark.True}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != starlark.True {
			t.Errorf("expected True, got %v", result)
		}
	})

	t.Run("handles string list arg", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Items []string `starlark:"items,required"`
		}

		fn := WrapFunc("process", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.MakeInt(len(args.Items)), nil
		})

		thread := &starlark.Thread{Name: "test"}
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.String("b"),
			starlark.String("c"),
		})
		result, err := starlark.Call(thread, fn, starlark.Tuple{list}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		i, _ := result.(starlark.Int).Int64()
		if i != 3 {
			t.Errorf("expected 3, got %d", i)
		}
	})

	t.Run("handles string map arg", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Headers map[string]string `starlark:"headers,required"`
		}

		fn := WrapFunc("request", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.String(args.Headers["Content-Type"]), nil
		})

		thread := &starlark.Thread{Name: "test"}
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.String("Content-Type"), starlark.String("application/json"))

		result, err := starlark.Call(thread, fn, starlark.Tuple{dict}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s := string(result.(starlark.String))
		if s != "application/json" {
			t.Errorf("expected 'application/json', got %q", s)
		}
	})

	t.Run("handles starlark.Value arg", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Value starlark.Value `starlark:"value,required"`
		}

		fn := WrapFunc("echo", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return args.Value, nil
		})

		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, fn, starlark.Tuple{starlark.MakeInt(42)}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		i, _ := result.(starlark.Int).Int64()
		if i != 42 {
			t.Errorf("expected 42, got %d", i)
		}
	})

	t.Run("returns error for missing required arg", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Name string `starlark:"name,required"`
		}

		fn := WrapFunc("greet", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.None, nil
		})

		thread := &starlark.Thread{Name: "test"}
		_, err := starlark.Call(thread, fn, nil, nil)
		if err == nil {
			t.Fatal("expected error for missing required arg")
		}
	})

	t.Run("returns error for wrong type", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Name string `starlark:"name,required"`
		}

		fn := WrapFunc("greet", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.None, nil
		})

		thread := &starlark.Thread{Name: "test"}
		_, err := starlark.Call(thread, fn, starlark.Tuple{starlark.MakeInt(42)}, nil)
		if err == nil {
			t.Fatal("expected error for wrong type")
		}
	})

	t.Run("returns error for unexpected positional arg", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Name string `starlark:"name,required"`
		}

		fn := WrapFunc("greet", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.None, nil
		})

		thread := &starlark.Thread{Name: "test"}
		_, err := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("hello"),
			starlark.String("extra"),
		}, nil)
		if err == nil {
			t.Fatal("expected error for extra positional arg")
		}
	})

	t.Run("returns error for unknown kwarg", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Name string `starlark:"name,required"`
		}

		fn := WrapFunc("greet", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.None, nil
		})

		thread := &starlark.Thread{Name: "test"}
		_, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("name"), starlark.String("test")},
			{starlark.String("unknown"), starlark.String("value")},
		})
		if err == nil {
			t.Fatal("expected error for unknown kwarg")
		}
	})

	t.Run("propagates function error", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Fail bool `starlark:"fail,required"`
		}

		fn := WrapFunc("maybe_fail", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			if args.Fail {
				return nil, errors.New("intentional failure")
			}
			return starlark.None, nil
		})

		thread := &starlark.Thread{Name: "test"}
		_, err := starlark.Call(thread, fn, starlark.Tuple{starlark.True}, nil)
		if err == nil {
			t.Fatal("expected error to be propagated")
		}
		if err.Error() != "intentional failure" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handles int64 arg", func(t *testing.T) {
		t.Parallel()
		type Args struct {
			Value int64 `starlark:"value,required"`
		}

		fn := WrapFunc("bignum", func(_ *starlark.Thread, args Args) (starlark.Value, error) {
			return starlark.MakeInt64(args.Value), nil
		})

		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, fn, starlark.Tuple{starlark.MakeInt64(9999999999)}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		i, _ := result.(starlark.Int).Int64()
		if i != 9999999999 {
			t.Errorf("expected 9999999999, got %d", i)
		}
	})
}

func TestWrapFuncPanics(t *testing.T) {
	t.Parallel()
	t.Run("panics on non-function", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for non-function")
			}
		}()
		WrapFunc("bad", "not a function")
	})

	t.Run("panics on wrong number of params", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for wrong param count")
			}
		}()
		WrapFunc("bad", func() (starlark.Value, error) {
			return starlark.None, nil
		})
	})

	t.Run("panics on wrong first param type", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for wrong first param")
			}
		}()
		type Args struct{}
		WrapFunc("bad", func(s string, args Args) (starlark.Value, error) {
			return starlark.None, nil
		})
	})

	t.Run("panics on non-struct second param", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for non-struct second param")
			}
		}()
		WrapFunc("bad", func(t *starlark.Thread, s string) (starlark.Value, error) {
			return starlark.None, nil
		})
	})
}

func TestNewBuiltin(t *testing.T) {
	t.Parallel()
	t.Run("creates simple builtin", func(t *testing.T) {
		t.Parallel()
		fn := NewBuiltin("add", func(_ *starlark.Thread, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
			if len(args) != 2 {
				return nil, errors.New("expected 2 args")
			}
			a, _ := args[0].(starlark.Int).Int64()
			b, _ := args[1].(starlark.Int).Int64()
			return starlark.MakeInt64(a + b), nil
		})

		thread := &starlark.Thread{Name: "test"}
		result, err := starlark.Call(thread, fn, starlark.Tuple{
			starlark.MakeInt(10),
			starlark.MakeInt(20),
		}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		i, _ := result.(starlark.Int).Int64()
		if i != 30 {
			t.Errorf("expected 30, got %d", i)
		}
	})
}
