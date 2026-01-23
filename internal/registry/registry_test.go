package registry

import (
	"testing"
)

func TestPathBuilders(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "ModulePath",
			fn:       func() string { return ModulePath("rules_go") },
			expected: "modules/rules_go",
		},
		{
			name:     "MetadataPath",
			fn:       func() string { return MetadataPath("rules_go") },
			expected: "modules/rules_go/metadata.json",
		},
		{
			name:     "VersionPath",
			fn:       func() string { return VersionPath("rules_go", "0.50.0") },
			expected: "modules/rules_go/0.50.0",
		},
		{
			name:     "ModuleBazelPath",
			fn:       func() string { return ModuleBazelPath("rules_go", "0.50.0") },
			expected: "modules/rules_go/0.50.0/MODULE.bazel",
		},
		{
			name:     "SourcePath",
			fn:       func() string { return SourcePath("rules_go", "0.50.0") },
			expected: "modules/rules_go/0.50.0/source.json",
		},
		{
			name:     "ModulesIndexPath",
			fn:       func() string { return ModulesIndexPath() },
			expected: "modules/index.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMetadata_LatestVersion(t *testing.T) {
	tests := []struct {
		name     string
		meta     Metadata
		expected string
	}{
		{
			name: "simple latest",
			meta: Metadata{
				Versions: []string{"1.0.0", "1.1.0", "1.2.0"},
			},
			expected: "1.2.0",
		},
		{
			name: "skip yanked",
			meta: Metadata{
				Versions:       []string{"1.0.0", "1.1.0", "1.2.0"},
				YankedVersions: map[string]string{"1.2.0": "buggy"},
			},
			expected: "1.1.0",
		},
		{
			name: "skip prerelease",
			meta: Metadata{
				Versions: []string{"1.0.0", "1.1.0", "1.2.0-rc1"},
			},
			expected: "1.1.0",
		},
		{
			name: "fallback to prerelease if only option",
			meta: Metadata{
				Versions:       []string{"1.0.0", "1.1.0-rc1"},
				YankedVersions: map[string]string{"1.0.0": "old"},
			},
			expected: "1.1.0-rc1",
		},
		{
			name: "all yanked returns empty",
			meta: Metadata{
				Versions:       []string{"1.0.0", "1.1.0"},
				YankedVersions: map[string]string{"1.0.0": "old", "1.1.0": "buggy"},
			},
			expected: "",
		},
		{
			name:     "empty versions",
			meta:     Metadata{},
			expected: "",
		},
		{
			name: "various prerelease formats",
			meta: Metadata{
				Versions: []string{"1.0.0", "2.0.0-alpha", "2.0.0-beta.1", "2.0.0-rc1"},
			},
			expected: "1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.meta.LatestVersion()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsPrerelease(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"1.0.0", false},
		{"1.0.0-rc1", true},
		{"1.0.0-alpha", true},
		{"1.0.0-beta", true},
		{"1.0.0-dev", true},
		{"1.0.0-pre", true},
		{"1.0.0.rc1", false}, // only dash-separated
		{"2.0.0-rc.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := isPrerelease(tt.version)
			if got != tt.expected {
				t.Errorf("isPrerelease(%q) = %v, want %v", tt.version, got, tt.expected)
			}
		})
	}
}
