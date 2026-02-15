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
	t.Parallel()
	all := All()

	assert.Contains(t, all, "glob")
	assert.Contains(t, all, "env")
	assert.Contains(t, all, "print")
	assert.Contains(t, all, "fail")
}

func TestGlob(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	t.Run("static matcher", func(t *testing.T) {
		t.Parallel()
		items := []string{"a", "b", "c"}
		matcher := NewStaticMatcher(items)
		assert.Equal(t, items, matcher.Candidates())
	})

	t.Run("set and get matcher", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		matcher := NewStaticMatcher([]string{"x"})

		SetGlobMatcher(thread, matcher)
		retrieved := GetGlobMatcher(thread)

		assert.Equal(t, matcher, retrieved)
	})

	t.Run("get matcher when not set", func(t *testing.T) {
		t.Parallel()
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
	t.Parallel()
	t.Run("set and get prefixes", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		prefixes := []string{"BZ_", "BAZEL_"}

		SetEnvAllowedPrefixes(thread, prefixes)
		retrieved := GetEnvAllowedPrefixes(thread)

		assert.Equal(t, prefixes, retrieved)
	})

	t.Run("get prefixes when not set", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		assert.Nil(t, GetEnvAllowedPrefixes(thread))
	})
}

func TestPrint(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	t.Run("is fail error", func(t *testing.T) {
		t.Parallel()
		err := &FailError{Message: "test"}
		assert.True(t, IsFailError(err))
	})

	t.Run("is not fail error", func(t *testing.T) {
		t.Parallel()
		err := errors.New("regular error")
		assert.False(t, IsFailError(err))
	})
}

func TestStruct(t *testing.T) {
	t.Parallel()
	t.Run("NewStruct", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
			result := ToStarlarkValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("string slice", func(t *testing.T) {
		t.Parallel()
		result := ToStarlarkValue([]string{"a", "b"})
		list := result.(*starlark.List)
		assert.Equal(t, 2, list.Len())
		s0, _ := starlark.AsString(list.Index(0))
		assert.Equal(t, "a", s0)
	})

	t.Run("interface slice", func(t *testing.T) {
		t.Parallel()
		result := ToStarlarkValue([]any{"a", 1, true})
		list := result.(*starlark.List)
		assert.Equal(t, 3, list.Len())
	})

	t.Run("map", func(t *testing.T) {
		t.Parallel()
		result := ToStarlarkValue(map[string]any{
			"key": "value",
		})
		dict := result.(*starlark.Dict)
		val, found, _ := dict.Get(starlark.String("key"))
		assert.True(t, found)
		assert.Equal(t, starlark.String("value"), val)
	})

	t.Run("starlark value passthrough", func(t *testing.T) {
		t.Parallel()
		original := starlark.String("test")
		result := ToStarlarkValue(original)
		assert.Equal(t, original, result)
	})
}

func TestFromStarlarkValue(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			result := FromStarlarkValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("list", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
		dict := starlark.NewDict(1)
		_ = dict.SetKey(starlark.String("key"), starlark.String("value"))

		result := FromStarlarkValue(dict)
		m := result.(map[string]any)
		assert.Equal(t, "value", m["key"])
	})
}

// --- Additional edge case tests ---

func TestGlob_MissingIncludeArg(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	SetGlobMatcher(thread, NewStaticMatcher([]string{"a", "b"}))

	globals := starlark.StringDict{"glob": Glob}

	// Calling glob() without include argument should fail
	_, err := starlark.Eval(thread, "test.star", `glob()`, globals)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "include")
}

func TestGlob_WrongTypeForInclude(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	SetGlobMatcher(thread, NewStaticMatcher([]string{"a"}))

	globals := starlark.StringDict{"glob": Glob}

	// Passing a string instead of list for include should fail
	_, err := starlark.Eval(thread, "test.star", `glob("*.go")`, globals)
	require.Error(t, err)
}

func TestGlob_NonStringInIncludeList(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	SetGlobMatcher(thread, NewStaticMatcher([]string{"a"}))

	globals := starlark.StringDict{"glob": Glob}

	_, err := starlark.Eval(thread, "test.star", `glob([42])`, globals)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "include")
}

func TestGlob_NonStringInExcludeList(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	SetGlobMatcher(thread, NewStaticMatcher([]string{"a"}))

	globals := starlark.StringDict{"glob": Glob}

	_, err := starlark.Eval(thread, "test.star", `glob(["*"], exclude=[42])`, globals)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exclude")
}

func TestGlob_EmptyCandidates(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	SetGlobMatcher(thread, NewStaticMatcher([]string{}))

	globals := starlark.StringDict{"glob": Glob}

	result, err := starlark.Eval(thread, "test.star", `glob(["*"])`, globals)
	require.NoError(t, err)

	list, ok := result.(*starlark.List)
	require.True(t, ok)
	assert.Equal(t, 0, list.Len())
}

func TestGlob_EmptyPatterns(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	SetGlobMatcher(thread, NewStaticMatcher([]string{"a", "b", "c"}))

	globals := starlark.StringDict{"glob": Glob}

	result, err := starlark.Eval(thread, "test.star", `glob([])`, globals)
	require.NoError(t, err)

	list, ok := result.(*starlark.List)
	require.True(t, ok)
	assert.Equal(t, 0, list.Len())
}

func TestGlob_ExcludeAll(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	SetGlobMatcher(thread, NewStaticMatcher([]string{"a.go", "b.go"}))

	globals := starlark.StringDict{"glob": Glob}

	result, err := starlark.Eval(thread, "test.star", `glob(["*.go"], exclude=["*.go"])`, globals)
	require.NoError(t, err)

	list, ok := result.(*starlark.List)
	require.True(t, ok)
	assert.Equal(t, 0, list.Len())
}

func TestEnv_MissingNameArg(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	globals := starlark.StringDict{"env": Env}

	_, err := starlark.Eval(thread, "test.star", `env()`, globals)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestEnv_WrongNameType(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	globals := starlark.StringDict{"env": Env}

	_, err := starlark.Eval(thread, "test.star", `env(42)`, globals)
	require.Error(t, err)
}

func TestEnv_NonStringDefault(t *testing.T) {
	// env() should accept non-string defaults because the default kwarg is
	// typed as starlark.Value
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	globals := starlark.StringDict{"env": Env}

	result, err := starlark.Eval(thread, "test.star", `env("BZ_VERY_UNLIKELY_SET_VAR_XYZ123", default=42)`, globals)
	require.NoError(t, err)
	i, ok := result.(starlark.Int)
	require.True(t, ok)
	v, _ := i.Int64()
	assert.Equal(t, int64(42), v)
}

func TestEnv_MultiplePrefixesAllowed(t *testing.T) {
	t.Setenv("BAZEL_TEST_KEY", "bazel_val")

	thread := &starlark.Thread{Name: "test"}
	SetEnvAllowedPrefixes(thread, []string{"BZ_", "BAZEL_"})

	globals := starlark.StringDict{"env": Env}

	result, err := starlark.Eval(thread, "test.star", `env("BAZEL_TEST_KEY")`, globals)
	require.NoError(t, err)

	s, ok := starlark.AsString(result)
	require.True(t, ok)
	assert.Equal(t, "bazel_val", s)
}

func TestEnv_NoPrefixes_AllowsAll(t *testing.T) {
	t.Setenv("ARBITRARY_VAR_FOR_TEST", "allowed")

	thread := &starlark.Thread{Name: "test"}
	// No prefix restrictions
	globals := starlark.StringDict{"env": Env}

	result, err := starlark.Eval(thread, "test.star", `env("ARBITRARY_VAR_FOR_TEST")`, globals)
	require.NoError(t, err)

	s, ok := starlark.AsString(result)
	require.True(t, ok)
	assert.Equal(t, "allowed", s)
}

func TestFail_WithKwargsError(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	globals := starlark.StringDict{"fail": Fail}

	_, err := starlark.Eval(thread, "test.star", `fail(msg="error")`, globals)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected keyword")
}

func TestFail_WithListArg(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	globals := starlark.StringDict{"fail": Fail}

	_, err := starlark.Eval(thread, "test.star", `fail("items:", [1, 2, 3])`, globals)
	require.Error(t, err)

	var failErr *FailError
	require.True(t, errors.As(err, &failErr))
	assert.Contains(t, failErr.Message, "items:")
	assert.Contains(t, failErr.Message, "[1, 2, 3]")
}

func TestFail_WithNone(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	globals := starlark.StringDict{"fail": Fail}

	_, err := starlark.Eval(thread, "test.star", `fail(None)`, globals)
	require.Error(t, err)

	var failErr *FailError
	require.True(t, errors.As(err, &failErr))
	assert.Equal(t, "None", failErr.Message)
}

func TestFailError_ErrorMethod(t *testing.T) {
	t.Parallel()

	e := &FailError{Message: "test failure"}
	assert.Equal(t, "test failure", e.Error())
}

func TestFailError_EmptyMessage(t *testing.T) {
	t.Parallel()

	e := &FailError{Message: ""}
	assert.Equal(t, "", e.Error())
}

func TestPrint_WithKwargs(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}

	var captured string
	SetPrintFunc(thread, func(_ *starlark.Thread, msg string) {
		captured = msg
	})

	globals := starlark.StringDict{"print": Print}

	_, err := starlark.Eval(thread, "test.star", `print("hello", name="world")`, globals)
	require.NoError(t, err)
	assert.Contains(t, captured, "hello")
	assert.Contains(t, captured, "name=world")
}

func TestPrint_WithList(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}

	var captured string
	SetPrintFunc(thread, func(_ *starlark.Thread, msg string) {
		captured = msg
	})

	globals := starlark.StringDict{"print": Print}

	_, err := starlark.Eval(thread, "test.star", `print([1, 2, 3])`, globals)
	require.NoError(t, err)
	assert.Equal(t, "[1, 2, 3]", captured)
}

func TestPrint_ReturnsNone(t *testing.T) {
	t.Parallel()

	thread := &starlark.Thread{Name: "test"}
	SetPrintFunc(thread, func(_ *starlark.Thread, _ string) {})

	globals := starlark.StringDict{"print": Print}

	result, err := starlark.Eval(thread, "test.star", `print("hello")`, globals)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, result)
}

func TestStruct_EmptyFields(t *testing.T) {
	t.Parallel()

	s := NewStruct("empty", map[string]starlark.Value{})
	assert.NotNil(t, s)
	names := s.AttrNames()
	assert.Empty(t, names)
}

func TestStructBuilder_ChainedSets(t *testing.T) {
	t.Parallel()

	s := NewStructBuilder("person").
		Set("name", "Alice").
		Set("age", 30).
		Set("active", true).
		Build()

	name, err := s.Attr("name")
	require.NoError(t, err)
	assert.Equal(t, starlark.String("Alice"), name)

	age, err := s.Attr("age")
	require.NoError(t, err)
	ageInt, _ := age.(starlark.Int).Int64()
	assert.Equal(t, int64(30), ageInt)

	active, err := s.Attr("active")
	require.NoError(t, err)
	assert.Equal(t, starlark.Bool(true), active)
}

func TestStructBuilder_SetNil(t *testing.T) {
	t.Parallel()

	s := NewStructBuilder("test").
		Set("null_field", nil).
		Build()

	val, err := s.Attr("null_field")
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

func TestStructBuilder_OverwriteField(t *testing.T) {
	t.Parallel()

	s := NewStructBuilder("test").
		Set("x", 1).
		Set("x", 2). // overwrite
		Build()

	val, err := s.Attr("x")
	require.NoError(t, err)
	i, _ := val.(starlark.Int).Int64()
	assert.Equal(t, int64(2), i)
}

func TestToStarlarkValue_FallbackToString(t *testing.T) {
	t.Parallel()

	// Unsupported type falls back to string representation
	type custom struct{ X int }
	result := ToStarlarkValue(custom{X: 42})
	s, ok := result.(starlark.String)
	require.True(t, ok)
	assert.Contains(t, string(s), "42")
}

func TestFromStarlarkValue_UnknownType(t *testing.T) {
	t.Parallel()

	// A custom starlark value type should be returned as-is
	type customValue struct{ starlark.Value }
	cv := &customValue{Value: starlark.String("inner")}
	result := FromStarlarkValue(cv)
	assert.Equal(t, cv, result)
}

func TestFromStarlarkValue_DictWithNonStringKey(t *testing.T) {
	t.Parallel()

	dict := starlark.NewDict(1)
	_ = dict.SetKey(starlark.MakeInt(1), starlark.String("value"))

	result := FromStarlarkValue(dict)
	// Non-string keys are silently skipped in FromStarlarkValue
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, m)
}

func TestAll_NoNilValues(t *testing.T) {
	t.Parallel()

	all := All()
	for name, val := range all {
		assert.NotNil(t, val, "builtin %q should not be nil", name)
	}
}

func TestGlob_StaticMatcher_EmptyList(t *testing.T) {
	t.Parallel()

	matcher := NewStaticMatcher(nil)
	assert.Nil(t, matcher.Candidates())
}

func TestIsAllowedEnvVar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		varName  string
		prefixes []string
		expected bool
	}{
		{"empty prefixes allows all", "HOME", nil, true},
		{"matching prefix", "BZ_TOKEN", []string{"BZ_"}, true},
		{"no matching prefix", "HOME", []string{"BZ_"}, false},
		{"multiple prefixes match first", "BZ_X", []string{"BZ_", "BAZEL_"}, true},
		{"multiple prefixes match second", "BAZEL_X", []string{"BZ_", "BAZEL_"}, true},
		{"empty var name", "", []string{"BZ_"}, false},
		{"exact prefix match", "BZ_", []string{"BZ_"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isAllowedEnvVar(tt.varName, tt.prefixes)
			assert.Equal(t, tt.expected, result)
		})
	}
}
