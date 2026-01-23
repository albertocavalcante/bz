package registry

import (
	"context"
	"sort"
	"strings"
)

// SearchResult represents a module found by search.
type SearchResult struct {
	Name string
	Meta *Metadata // May be nil if metadata wasn't fetched
}

// Search finds modules matching the query using fuzzy name matching.
// Results are sorted by relevance (prefix matches first, then alphabetically).
func Search(ctx context.Context, reg Registry, query string) ([]SearchResult, error) {
	modules, err := reg.ListModules(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []SearchResult

	for _, name := range modules {
		nameLower := strings.ToLower(name)
		if matchesQuery(nameLower, query) {
			results = append(results, SearchResult{Name: name})
		}
	}

	// Sort by relevance
	sort.Slice(results, func(i, j int) bool {
		ni := strings.ToLower(results[i].Name)
		nj := strings.ToLower(results[j].Name)

		// Exact match first
		if ni == query && nj != query {
			return true
		}
		if nj == query && ni != query {
			return false
		}

		// Prefix match second
		iPre := strings.HasPrefix(ni, query)
		jPre := strings.HasPrefix(nj, query)
		if iPre != jPre {
			return iPre
		}

		// Alphabetical
		return ni < nj
	})

	return results, nil
}

// matchesQuery checks if a name matches the search query.
func matchesQuery(name, query string) bool {
	// Direct substring match
	if strings.Contains(name, query) {
		return true
	}

	// Match all parts (e.g., "rules go" matches "rules_go")
	return matchesParts(name, query)
}

// matchesParts checks if name contains all parts of query split by separators.
func matchesParts(name, query string) bool {
	parts := splitByAny(query, "_- ")
	if len(parts) <= 1 {
		return false
	}

	for _, part := range parts {
		if part != "" && !strings.Contains(name, part) {
			return false
		}
	}
	return true
}

// splitByAny splits a string by any of the given separator characters.
func splitByAny(s string, seps string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return strings.ContainsRune(seps, r)
	})
}
