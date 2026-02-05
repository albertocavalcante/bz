package registry

import (
	"context"
	"errors"
	"sort"
	"strings"
)

// MaxSuggestionDistance is the maximum Levenshtein distance for a suggestion to be considered.
// This prevents suggesting completely unrelated modules.
const MaxSuggestionDistance = 5

// LevenshteinDistance computes the Levenshtein distance between two strings.
// The Levenshtein distance is the minimum number of single-character edits
// (insertions, deletions, or substitutions) required to change one string into the other.
func LevenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create a matrix to store distances
	// We only need two rows at a time, so we optimize space
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	// Initialize the first row
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	// Fill in the rest of the matrix
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = min(
				curr[j-1]+1,    // insertion
				prev[j]+1,      // deletion
				prev[j-1]+cost, // substitution
			)
		}
		// Swap rows
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

// suggestion holds a module name and its distance from the query.
type suggestion struct {
	name     string
	distance int
}

// FindSuggestions finds module names similar to the query using Levenshtein distance.
// It returns up to maxResults suggestions sorted by similarity (closest first).
// If the query exactly matches a module, no suggestions are returned.
// Suggestions are case-insensitive.
func FindSuggestions(query string, modules []string, maxResults int) []string {
	if query == "" || len(modules) == 0 || maxResults <= 0 {
		return []string{}
	}

	queryLower := strings.ToLower(query)

	var suggestions []suggestion
	for _, mod := range modules {
		modLower := strings.ToLower(mod)

		// Skip exact matches
		if modLower == queryLower {
			return []string{}
		}

		distance := LevenshteinDistance(queryLower, modLower)

		// Only consider suggestions within the threshold
		// Use a dynamic threshold based on query length
		threshold := maxThreshold(queryLower)
		if distance <= threshold {
			suggestions = append(suggestions, suggestion{
				name:     mod,
				distance: distance,
			})
		}
	}

	// Sort by distance (closest first), then alphabetically for ties
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].distance != suggestions[j].distance {
			return suggestions[i].distance < suggestions[j].distance
		}
		return suggestions[i].name < suggestions[j].name
	})

	// Limit results
	if len(suggestions) > maxResults {
		suggestions = suggestions[:maxResults]
	}

	// Extract names
	result := make([]string, len(suggestions))
	for i, s := range suggestions {
		result[i] = s.name
	}

	return result
}

// maxThreshold calculates the maximum edit distance threshold based on query length.
// Shorter queries need a smaller threshold to avoid too many false positives.
func maxThreshold(query string) int {
	length := len(query)
	switch {
	case length <= 3:
		return 1
	case length <= 6:
		return 2
	case length <= 10:
		return 3
	default:
		// For longer strings, allow more edits but cap at MaxSuggestionDistance
		threshold := length / 3
		if threshold > MaxSuggestionDistance {
			return MaxSuggestionDistance
		}
		return threshold
	}
}

// FormatSuggestion formats a list of suggestions into a human-readable string.
// Returns an empty string if there are no suggestions.
func FormatSuggestion(suggestions []string) string {
	if len(suggestions) == 0 {
		return ""
	}
	if len(suggestions) == 1 {
		return "Did you mean: " + suggestions[0] + "?"
	}
	return "Did you mean one of: " + strings.Join(suggestions, ", ") + "?"
}

// min returns the minimum of the given integers.
func min(nums ...int) int {
	m := nums[0]
	for _, n := range nums[1:] {
		if n < m {
			m = n
		}
	}
	return m
}

// ModuleNotFoundError wraps ErrModuleNotFound with the module name and optional suggestions.
type ModuleNotFoundError struct {
	Module      string
	Suggestions []string
}

// Error implements the error interface.
func (e *ModuleNotFoundError) Error() string {
	msg := "module \"" + e.Module + "\" not found"
	if suggestion := FormatSuggestion(e.Suggestions); suggestion != "" {
		msg += "\n" + suggestion
	}
	return msg
}

// Unwrap returns the underlying error for errors.Is() compatibility.
func (e *ModuleNotFoundError) Unwrap() error {
	return ErrModuleNotFound
}

// WrapModuleNotFound wraps an ErrModuleNotFound error with suggestions from the registry.
// If the error is not ErrModuleNotFound, it returns the original error unchanged.
// If the registry doesn't support listing or listing fails, it returns a simple ModuleNotFoundError.
func WrapModuleNotFound(ctx context.Context, reg Registry, moduleName string, err error) error {
	if !errors.Is(err, ErrModuleNotFound) {
		return err
	}

	result := &ModuleNotFoundError{
		Module: moduleName,
	}

	// Try to get suggestions from the registry
	modules, listErr := reg.ListModules(ctx)
	if listErr != nil {
		// If listing is not supported or fails, return without suggestions
		return result
	}

	result.Suggestions = FindSuggestions(moduleName, modules, 3)
	return result
}
