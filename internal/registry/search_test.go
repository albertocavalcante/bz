package registry

import (
	"context"
	"testing"

	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestSearch(t *testing.T) {
	root := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions:       []string{"0.49.0", "0.50.0", "0.51.0-rc1"},
			YankedVersions: map[string]string{"0.49.0": "buggy"},
		},
		"protobuf": {Versions: []string{"3.19.0", "3.20.0"}},
	})
	reg := NewFileRegistry(root)
	ctx := context.Background()

	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "exact match",
			query:    "rules_go",
			expected: []string{"rules_go"},
		},
		{
			name:     "substring match",
			query:    "rules",
			expected: []string{"rules_go"},
		},
		{
			name:     "case insensitive",
			query:    "RULES_GO",
			expected: []string{"rules_go"},
		},
		{
			name:     "partial match",
			query:    "proto",
			expected: []string{"protobuf"},
		},
		{
			name:     "no match",
			query:    "nonexistent",
			expected: []string{},
		},
		{
			name:     "parts match",
			query:    "rules go",
			expected: []string{"rules_go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := Search(ctx, reg, tt.query)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}

			if len(results) != len(tt.expected) {
				names := make([]string, len(results))
				for i, r := range results {
					names[i] = r.Name
				}
				t.Errorf("Search() returned %v, want %v", names, tt.expected)
				return
			}

			for i, exp := range tt.expected {
				if results[i].Name != exp {
					t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, exp)
				}
			}
		})
	}
}

func TestSearch_Relevance(t *testing.T) {
	root := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"grpc_rules":  {Versions: []string{"1.0.0"}},
		"rules_grpc":  {Versions: []string{"1.0.0"}},
		"grpc":        {Versions: []string{"1.0.0"}},
		"my_grpc_lib": {Versions: []string{"1.0.0"}},
	})
	reg := NewFileRegistry(root)
	ctx := context.Background()

	results, err := Search(ctx, reg, "grpc")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// Expected order: exact match first, then prefix, then alphabetical
	expected := []string{"grpc", "grpc_rules", "my_grpc_lib", "rules_grpc"}

	if len(results) != len(expected) {
		t.Fatalf("got %d results, want %d", len(results), len(expected))
	}

	for i, exp := range expected {
		if results[i].Name != exp {
			names := make([]string, len(results))
			for j, r := range results {
				names[j] = r.Name
			}
			t.Errorf("Search() = %v, want %v", names, expected)
			break
		}
	}
}
