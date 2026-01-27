package builtins

import (
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
	"go.starlark.net/starlark"
)

// Glob is a Starlark builtin that matches patterns against a list of strings.
// This is inspired by Bazel's glob() function but operates on in-memory string lists
// rather than filesystem paths, making it suitable for module name matching.
//
// Usage in Starlark:
//
//	files = glob(["*.go", "**/*.star"])
//	files = glob(["src/**"], exclude=["*_test.go"])
//	modules = glob(["rules_*", "bazel_skylib"])
var Glob = starlark.NewBuiltin("glob", globImpl)

// GlobMatcher defines the interface for matching candidates against patterns.
// This allows the glob function to work with different sources of candidates.
type GlobMatcher interface {
	// Candidates returns the list of strings to match against.
	Candidates() []string
}

// globMatcherKey is the thread-local key for storing a GlobMatcher.
const globMatcherKey = "glob_matcher"

// SetGlobMatcher sets the GlobMatcher for a Starlark thread.
// This must be called before executing Starlark code that uses glob().
func SetGlobMatcher(thread *starlark.Thread, matcher GlobMatcher) {
	thread.SetLocal(globMatcherKey, matcher)
}

// GetGlobMatcher retrieves the GlobMatcher from a Starlark thread.
func GetGlobMatcher(thread *starlark.Thread) GlobMatcher {
	if v := thread.Local(globMatcherKey); v != nil {
		if m, ok := v.(GlobMatcher); ok {
			return m
		}
	}
	return nil
}

// StaticMatcher is a GlobMatcher that uses a fixed list of candidates.
type StaticMatcher struct {
	items []string
}

// NewStaticMatcher creates a GlobMatcher with a fixed list of candidates.
func NewStaticMatcher(items []string) *StaticMatcher {
	return &StaticMatcher{items: items}
}

// Candidates returns the list of strings to match against.
func (m *StaticMatcher) Candidates() []string {
	return m.items
}

func globImpl(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var include *starlark.List
	var exclude *starlark.List

	if err := starlark.UnpackArgs("glob", args, kwargs,
		"include", &include,
		"exclude?", &exclude,
	); err != nil {
		return nil, err
	}

	// Get the matcher from thread-local storage
	matcher := GetGlobMatcher(thread)
	if matcher == nil {
		return nil, fmt.Errorf("glob: no matcher configured (call SetGlobMatcher before execution)")
	}

	// Convert include patterns to Go strings
	includePatterns, err := listToStrings(include, "include")
	if err != nil {
		return nil, err
	}

	// Convert exclude patterns to Go strings (if provided)
	var excludePatterns []string
	if exclude != nil {
		excludePatterns, err = listToStrings(exclude, "exclude")
		if err != nil {
			return nil, err
		}
	}

	// Match candidates against patterns
	candidates := matcher.Candidates()
	var results []starlark.Value

	for _, candidate := range candidates {
		if matchesAny(candidate, includePatterns) && !matchesAny(candidate, excludePatterns) {
			results = append(results, starlark.String(candidate))
		}
	}

	return starlark.NewList(results), nil
}

// listToStrings converts a Starlark list to a Go string slice.
func listToStrings(list *starlark.List, name string) ([]string, error) {
	if list == nil {
		return nil, nil
	}

	result := make([]string, list.Len())
	for i := 0; i < list.Len(); i++ {
		s, ok := starlark.AsString(list.Index(i))
		if !ok {
			return nil, fmt.Errorf("%s[%d]: expected string, got %s", name, i, list.Index(i).Type())
		}
		result[i] = s
	}
	return result, nil
}

// matchesAny checks if a candidate matches any of the given patterns.
func matchesAny(candidate string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, candidate)
		if err == nil && matched {
			return true
		}
	}
	return false
}
