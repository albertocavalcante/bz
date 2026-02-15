package registry

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		{
			name:     "identical strings",
			a:        "rules_go",
			b:        "rules_go",
			expected: 0,
		},
		{
			name:     "single character difference",
			a:        "rule_go",
			b:        "rules_go",
			expected: 1,
		},
		{
			name:     "two character difference",
			a:        "rulz_python",
			b:        "rules_python",
			expected: 2,
		},
		{
			name:     "empty string",
			a:        "",
			b:        "test",
			expected: 4,
		},
		{
			name:     "both empty",
			a:        "",
			b:        "",
			expected: 0,
		},
		{
			name:     "completely different",
			a:        "abc",
			b:        "xyz",
			expected: 3,
		},
		{
			name:     "case sensitive",
			a:        "Rules_Go",
			b:        "rules_go",
			expected: 2,
		},
		{
			name:     "transposition",
			a:        "ruels_go",
			b:        "rules_go",
			expected: 2, // Levenshtein treats transposition as 2 ops
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := LevenshteinDistance(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("LevenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestFindSuggestions(t *testing.T) {
	t.Parallel()
	modules := []string{
		"rules_go",
		"rules_python",
		"rules_java",
		"gazelle",
		"protobuf",
		"rules_pkg",
		"rules_docker",
	}

	tests := []struct {
		name       string
		query      string
		modules    []string
		maxResults int
		expected   []string
	}{
		{
			name:       "missing s in rules",
			query:      "rule_go",
			modules:    modules,
			maxResults: 3,
			expected:   []string{"rules_go"},
		},
		{
			name:       "typo in rules",
			query:      "rulz_python",
			modules:    modules,
			maxResults: 3,
			expected:   []string{"rules_python"},
		},
		{
			name:       "multiple suggestions with similar distance",
			query:      "rules_goo",
			modules:    modules,
			maxResults: 3,
			expected:   []string{"rules_go", "rules_pkg"},
		},
		{
			name:       "typo in gazelle",
			query:      "gazel",
			modules:    modules,
			maxResults: 3,
			expected:   []string{"gazelle"},
		},
		{
			name:       "no close matches",
			query:      "completely_different_name",
			modules:    modules,
			maxResults: 3,
			expected:   []string{},
		},
		{
			name:       "exact match returns nothing",
			query:      "rules_go",
			modules:    modules,
			maxResults: 3,
			expected:   []string{},
		},
		{
			name:       "empty query",
			query:      "",
			modules:    modules,
			maxResults: 3,
			expected:   []string{},
		},
		{
			name:       "empty modules list",
			query:      "rules_go",
			modules:    []string{},
			maxResults: 3,
			expected:   []string{},
		},
		{
			name:       "limit results",
			query:      "rules_goo",
			modules:    modules,
			maxResults: 1,
			expected:   []string{"rules_go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FindSuggestions(tt.query, tt.modules, tt.maxResults)
			if len(result) != len(tt.expected) {
				t.Errorf("FindSuggestions(%q) = %v, want %v", tt.query, result, tt.expected)
				return
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("FindSuggestions(%q)[%d] = %q, want %q", tt.query, i, result[i], exp)
				}
			}
		})
	}
}

func TestFindSuggestionsCaseInsensitive(t *testing.T) {
	t.Parallel()
	modules := []string{"rules_go", "Rules_Python", "GAZELLE"}

	// Query with different case should still find suggestions
	result := FindSuggestions("RULE_GO", modules, 3)
	if len(result) == 0 {
		t.Error("FindSuggestions should be case insensitive and find suggestions")
	}
	if len(result) > 0 && result[0] != "rules_go" {
		t.Errorf("FindSuggestions(\"RULE_GO\") = %v, want [\"rules_go\"]", result)
	}
}

func TestSuggestionThreshold(t *testing.T) {
	t.Parallel()
	modules := []string{"rules_go", "rules_python"}

	// A query that's too different shouldn't suggest anything
	result := FindSuggestions("xyz123", modules, 3)
	if len(result) != 0 {
		t.Errorf("FindSuggestions for very different query should return empty, got %v", result)
	}

	// A query that's close should suggest
	result = FindSuggestions("rules_goo", modules, 3)
	if len(result) == 0 {
		t.Error("FindSuggestions for close query should return suggestions")
	}
}

func TestFormatSuggestion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		suggestions []string
		expected    string
	}{
		{
			name:        "single suggestion",
			suggestions: []string{"rules_go"},
			expected:    "Did you mean: rules_go?",
		},
		{
			name:        "multiple suggestions",
			suggestions: []string{"rules_go", "rules_python"},
			expected:    "Did you mean one of: rules_go, rules_python?",
		},
		{
			name:        "no suggestions",
			suggestions: []string{},
			expected:    "",
		},
		{
			name:        "nil suggestions",
			suggestions: nil,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FormatSuggestion(tt.suggestions)
			if result != tt.expected {
				t.Errorf("FormatSuggestion(%v) = %q, want %q", tt.suggestions, result, tt.expected)
			}
		})
	}
}

func TestModuleNotFoundError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		module      string
		suggestions []string
		wantError   string
	}{
		{
			name:        "without suggestions",
			module:      "rule_go",
			suggestions: nil,
			wantError:   `module "rule_go" not found`,
		},
		{
			name:        "with single suggestion",
			module:      "rule_go",
			suggestions: []string{"rules_go"},
			wantError:   "module \"rule_go\" not found\nDid you mean: rules_go?",
		},
		{
			name:        "with multiple suggestions",
			module:      "rulz_python",
			suggestions: []string{"rules_python", "rules_go"},
			wantError:   "module \"rulz_python\" not found\nDid you mean one of: rules_python, rules_go?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := &ModuleNotFoundError{
				Module:      tt.module,
				Suggestions: tt.suggestions,
			}

			if err.Error() != tt.wantError {
				t.Errorf("ModuleNotFoundError.Error() = %q, want %q", err.Error(), tt.wantError)
			}

			// Verify errors.Is works with ErrModuleNotFound
			if !errors.Is(err, ErrModuleNotFound) {
				t.Error("ModuleNotFoundError should be unwrappable to ErrModuleNotFound")
			}
		})
	}
}

func TestWrapModuleNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock registry with modules
	mockReg := NewMockRegistry(map[string]*Metadata{
		"rules_go":     {Versions: []string{"0.50.1"}},
		"rules_python": {Versions: []string{"0.35.0"}},
		"gazelle":      {Versions: []string{"0.38.0"}},
	})

	t.Run("wraps ErrModuleNotFound with suggestions", func(t *testing.T) {
		t.Parallel()
		err := WrapModuleNotFound(ctx, mockReg, "rule_go", ErrModuleNotFound)

		var notFoundErr *ModuleNotFoundError
		if !errors.As(err, &notFoundErr) {
			t.Fatal("expected ModuleNotFoundError")
		}

		if notFoundErr.Module != "rule_go" {
			t.Errorf("Module = %q, want %q", notFoundErr.Module, "rule_go")
		}

		if len(notFoundErr.Suggestions) == 0 {
			t.Error("expected suggestions for 'rule_go'")
		}

		if notFoundErr.Suggestions[0] != "rules_go" {
			t.Errorf("first suggestion = %q, want %q", notFoundErr.Suggestions[0], "rules_go")
		}

		// Should still be compatible with errors.Is
		if !errors.Is(err, ErrModuleNotFound) {
			t.Error("wrapped error should still be ErrModuleNotFound")
		}
	})

	t.Run("passes through non-module-not-found errors", func(t *testing.T) {
		t.Parallel()
		otherErr := fmt.Errorf("some other error")
		err := WrapModuleNotFound(ctx, mockReg, "rule_go", otherErr)

		if err != otherErr {
			t.Error("expected original error to be returned unchanged")
		}
	})

	t.Run("handles no close matches", func(t *testing.T) {
		t.Parallel()
		err := WrapModuleNotFound(ctx, mockReg, "completely_different_name", ErrModuleNotFound)

		var notFoundErr *ModuleNotFoundError
		if !errors.As(err, &notFoundErr) {
			t.Fatal("expected ModuleNotFoundError")
		}

		if len(notFoundErr.Suggestions) != 0 {
			t.Errorf("expected no suggestions, got %v", notFoundErr.Suggestions)
		}
	})
}
