package builtins

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestAll(t *testing.T) {
	all := All()

	assert.Contains(t, all, "glob")
	assert.Contains(t, all, "env")
	assert.Contains(t, all, "print")
	assert.Contains(t, all, "fail")
}

func TestGlob(t *testing.T) {
	tests := []struct {
		name       string
		candidates []string
		code       string
		expected   []string
		wantErr    string
	}{
		{
			name:       "simple pattern",
			candidates: []string{"rules_go", "rules_python", "bazel_skylib"},
			code:       `glob(["rules_*"])`,
			expected:   []string{"rules_go", "rules_python"},
		},
		{
			name:       "exact match",
			candidates: []string{"rules_go", "rules_python", "bazel_skylib"},
			code:       `glob(["bazel_skylib"])`,
			expected:   []string{"bazel_skylib"},
		},
		{
			name:       "multiple patterns",
			candidates: []string{"rules_go", "rules_python", "bazel_skylib", "gazelle"},
			code:       `glob(["rules_*", "gazelle"])`,
			expected:   []string{"rules_go", "rules_python", "gazelle"},
		},
		{
			name:       "with exclude",
			candidates: []string{"rules_go", "rules_python", "rules_java", "bazel_skylib"},
			code:       `glob(["rules_*"], exclude=["rules_java"])`,
			expected:   []string{"rules_go", "rules_python"},
		},
		{
			name:       "exclude pattern",
			candidates: []string{"foo_test.go", "bar_test.go", "main.go", "util.go"},
			code:       `glob(["*.go"], exclude=["*_test.go"])`,
			expected:   []string{"main.go", "util.go"},
		},
		{
			name:       "no matches",
			candidates: []string{"rules_go", "rules_python"},
			code:       `glob(["bazel_*"])`,
			expected:   []string{},
		},
		{
			name:       "doublestar pattern",
			candidates: []string{"src/main.go", "src/util/helper.go", "test/main_test.go"},
			code:       `glob(["src/**/*.go"])`,
			expected:   []string{"src/main.go", "src/util/helper.go"},
		},
		{
			name:       "match all",
			candidates: []string{"a", "b", "c"},
			code:       `glob(["*"])`,
			expected:   []string{"a", "b", "c"},
		},
		{
			name:       "no matcher configured",
			candidates: nil, // Will not set matcher
			code:       `glob(["*"])`,
			wantErr:    "no matcher configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}

			// Set up matcher if candidates are provided
			if tt.candidates != nil {
				SetGlobMatcher(thread, NewStaticMatcher(tt.candidates))
			}

			globals := starlark.StringDict{"glob": Glob}

			result, err := starlark.Eval(thread, "test.star", tt.code, globals)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)

			list, ok := result.(*starlark.List)
			require.True(t, ok, "expected list, got %T", result)

			var actual []string
			for i := 0; i < list.Len(); i++ {
				s, _ := starlark.AsString(list.Index(i))
				actual = append(actual, s)
			}

			assert.ElementsMatch(t, tt.expected, actual)
		})
	}
}

func TestGlobMatcher(t *testing.T) {
	t.Run("static matcher", func(t *testing.T) {
		items := []string{"a", "b", "c"}
		matcher := NewStaticMatcher(items)
		assert.Equal(t, items, matcher.Candidates())
	})

	t.Run("set and get matcher", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		matcher := NewStaticMatcher([]string{"x"})

		SetGlobMatcher(thread, matcher)
		retrieved := GetGlobMatcher(thread)

		assert.Equal(t, matcher, retrieved)
	})

	t.Run("get matcher when not set", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		assert.Nil(t, GetGlobMatcher(thread))
	})
}

func TestEnv(t *testing.T) {
	// Set up test environment variables
	t.Setenv("BZ_TEST_VAR", "test_value")
	t.Setenv("BZ_TEST_EMPTY", "")

	tests := []struct {
		name     string
		code     string
		prefixes []string
		expected string
		wantErr  string
	}{
		{
			name:     "get existing var",
			code:     `env("BZ_TEST_VAR")`,
			expected: "test_value",
		},
		{
			name:     "get existing var with default",
			code:     `env("BZ_TEST_VAR", default="fallback")`,
			expected: "test_value",
		},
		{
			name:     "get missing var with default",
			code:     `env("BZ_NONEXISTENT", default="fallback")`,
			expected: "fallback",
		},
		{
			name:    "get missing var without default",
			code:    `env("BZ_NONEXISTENT")`,
			wantErr: "is not set",
		},
		{
			name:     "get empty var",
			code:     `env("BZ_TEST_EMPTY")`,
			expected: "",
		},
		{
			name:     "prefix restriction allowed",
			code:     `env("BZ_TEST_VAR")`,
			prefixes: []string{"BZ_"},
			expected: "test_value",
		},
		{
			name:     "prefix restriction denied",
			code:     `env("HOME")`,
			prefixes: []string{"BZ_"},
			wantErr:  "not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}

			if len(tt.prefixes) > 0 {
				SetEnvAllowedPrefixes(thread, tt.prefixes)
			}

			globals := starlark.StringDict{"env": Env}

			result, err := starlark.Eval(thread, "test.star", tt.code, globals)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)

			s, ok := starlark.AsString(result)
			require.True(t, ok, "expected string, got %T", result)
			assert.Equal(t, tt.expected, s)
		})
	}
}

func TestEnvAllowedPrefixes(t *testing.T) {
	t.Run("set and get prefixes", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		prefixes := []string{"BZ_", "BAZEL_"}

		SetEnvAllowedPrefixes(thread, prefixes)
		retrieved := GetEnvAllowedPrefixes(thread)

		assert.Equal(t, prefixes, retrieved)
	})

	t.Run("get prefixes when not set", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		assert.Nil(t, GetEnvAllowedPrefixes(thread))
	})
}

func TestPrint(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "simple string",
			code:     `print("hello")`,
			expected: "hello",
		},
		{
			name:     "multiple args",
			code:     `print("hello", "world")`,
			expected: "hello world",
		},
		{
			name:     "mixed types",
			code:     `print("count:", 42, "flag:", True)`,
			expected: "count: 42 flag: True",
		},
		{
			name:     "no args",
			code:     `print()`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}

			var captured string
			SetPrintFunc(thread, func(_ *starlark.Thread, msg string) {
				captured = msg
			})

			globals := starlark.StringDict{"print": Print}

			_, err := starlark.Eval(thread, "test.star", tt.code, globals)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, captured)
		})
	}
}

func TestPrintFunc(t *testing.T) {
	t.Run("set and get print func", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}

		called := false
		fn := func(_ *starlark.Thread, _ string) {
			called = true
		}

		SetPrintFunc(thread, fn)
		retrieved := GetPrintFunc(thread)

		require.NotNil(t, retrieved)
		retrieved(thread, "test")
		assert.True(t, called)
	})

	t.Run("get print func when not set", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		assert.Nil(t, GetPrintFunc(thread))
	})

	t.Run("default print to stdout", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		thread := &starlark.Thread{Name: "test"}
		globals := starlark.StringDict{"print": Print}

		_, err := starlark.Eval(thread, "test.star", `print("stdout test")`, globals)
		require.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		assert.Contains(t, string(buf[:n]), "stdout test")
	})
}

func TestFail(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "simple message",
			code:     `fail("something went wrong")`,
			expected: "something went wrong",
		},
		{
			name:     "multiple args",
			code:     `fail("error:", "invalid", "config")`,
			expected: "error: invalid config",
		},
		{
			name:     "no args",
			code:     `fail()`,
			expected: "fail() called",
		},
		{
			name:     "mixed types",
			code:     `fail("count is", 42)`,
			expected: "count is 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			globals := starlark.StringDict{"fail": Fail}

			_, err := starlark.Eval(thread, "test.star", tt.code, globals)

			require.Error(t, err)

			// Check that it's a FailError
			var failErr *FailError
			require.True(t, errors.As(err, &failErr), "expected FailError, got %T", err)
			assert.Equal(t, tt.expected, failErr.Message)
		})
	}
}

func TestIsFailError(t *testing.T) {
	t.Run("is fail error", func(t *testing.T) {
		err := &FailError{Message: "test"}
		assert.True(t, IsFailError(err))
	})

	t.Run("is not fail error", func(t *testing.T) {
		err := errors.New("regular error")
		assert.False(t, IsFailError(err))
	})
}

func TestStruct(t *testing.T) {
	t.Run("NewStruct", func(t *testing.T) {
		s := NewStruct("module", map[string]starlark.Value{
			"name":    starlark.String("rules_go"),
			"version": starlark.String("0.50.0"),
		})

		name, err := s.Attr("name")
		require.NoError(t, err)
		assert.Equal(t, starlark.String("rules_go"), name)

		version, err := s.Attr("version")
		require.NoError(t, err)
		assert.Equal(t, starlark.String("0.50.0"), version)
	})

	t.Run("StructBuilder", func(t *testing.T) {
		s := NewStructBuilder("config").
			Set("name", "test").
			Set("count", 42).
			Set("enabled", true).
			Set("tags", []string{"a", "b"}).
			Build()

		name, _ := s.Attr("name")
		assert.Equal(t, starlark.String("test"), name)

		count, _ := s.Attr("count")
		countInt, _ := count.(starlark.Int).Int64()
		assert.Equal(t, int64(42), countInt)

		enabled, _ := s.Attr("enabled")
		assert.Equal(t, starlark.Bool(true), enabled)

		tags, _ := s.Attr("tags")
		tagsList := tags.(*starlark.List)
		assert.Equal(t, 2, tagsList.Len())
	})

	t.Run("StructBuilder SetValue", func(t *testing.T) {
		nested := NewStruct("inner", map[string]starlark.Value{
			"x": starlark.MakeInt(1),
		})

		s := NewStructBuilder("outer").
			SetValue("inner", nested).
			Build()

		inner, _ := s.Attr("inner")
		assert.NotNil(t, inner)
	})
}

func TestToStarlarkValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected starlark.Value
	}{
		{"nil", nil, starlark.None},
		{"bool true", true, starlark.Bool(true)},
		{"bool false", false, starlark.Bool(false)},
		{"int", 42, starlark.MakeInt(42)},
		{"int64", int64(42), starlark.MakeInt64(42)},
		{"float64", 3.14, starlark.Float(3.14)},
		{"string", "hello", starlark.String("hello")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToStarlarkValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("string slice", func(t *testing.T) {
		result := ToStarlarkValue([]string{"a", "b"})
		list := result.(*starlark.List)
		assert.Equal(t, 2, list.Len())
		s0, _ := starlark.AsString(list.Index(0))
		assert.Equal(t, "a", s0)
	})

	t.Run("interface slice", func(t *testing.T) {
		result := ToStarlarkValue([]any{"a", 1, true})
		list := result.(*starlark.List)
		assert.Equal(t, 3, list.Len())
	})

	t.Run("map", func(t *testing.T) {
		result := ToStarlarkValue(map[string]any{
			"key": "value",
		})
		dict := result.(*starlark.Dict)
		val, found, _ := dict.Get(starlark.String("key"))
		assert.True(t, found)
		assert.Equal(t, starlark.String("value"), val)
	})

	t.Run("starlark value passthrough", func(t *testing.T) {
		original := starlark.String("test")
		result := ToStarlarkValue(original)
		assert.Equal(t, original, result)
	})
}

func TestFromStarlarkValue(t *testing.T) {
	tests := []struct {
		name     string
		input    starlark.Value
		expected any
	}{
		{"none", starlark.None, nil},
		{"bool true", starlark.Bool(true), true},
		{"bool false", starlark.Bool(false), false},
		{"int", starlark.MakeInt(42), int64(42)},
		{"float", starlark.Float(3.14), 3.14},
		{"string", starlark.String("hello"), "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromStarlarkValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("list", func(t *testing.T) {
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.MakeInt(1),
		})
		result := FromStarlarkValue(list)
		slice := result.([]any)
		assert.Equal(t, "a", slice[0])
		assert.Equal(t, int64(1), slice[1])
	})

	t.Run("dict", func(t *testing.T) {
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.String("key"), starlark.String("value"))

		result := FromStarlarkValue(dict)
		m := result.(map[string]any)
		assert.Equal(t, "value", m["key"])
	})
}
