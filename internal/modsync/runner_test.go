package modsync

import (
	"testing"
)

func TestLatestN_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		versions []string
		n        int
		want     []string
	}{
		{
			name:     "last 2 of 5",
			versions: []string{"1.0.0", "1.1.0", "1.2.0", "1.3.0", "1.4.0"},
			n:        2,
			want:     []string{"1.3.0", "1.4.0"},
		},
		{
			name:     "n equals length returns all",
			versions: []string{"1.0.0", "2.0.0", "3.0.0"},
			n:        3,
			want:     []string{"1.0.0", "2.0.0", "3.0.0"},
		},
		{
			name:     "nil input",
			versions: nil,
			n:        2,
			want:     nil,
		},
		{
			name:     "n is zero returns empty tail slice",
			versions: []string{"1.0.0", "2.0.0"},
			n:        0,
			want:     []string{},
		},
		{
			name:     "single element n=1",
			versions: []string{"1.0.0"},
			n:        1,
			want:     []string{"1.0.0"},
		},
		{
			name:     "n exactly one less than length",
			versions: []string{"a", "b", "c", "d"},
			n:        3,
			want:     []string{"b", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := latestN(tt.versions, tt.n)

			if !stringSliceEqual(got, tt.want) {
				t.Errorf("latestN(%v, %d) = %v, want %v", tt.versions, tt.n, got, tt.want)
			}
		})
	}
}

func TestFilterSince_Passthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		versions []string
		since    string
		want     []string
	}{
		{
			name:     "returns all versions unchanged",
			versions: []string{"1.0.0", "1.1.0", "1.2.0"},
			since:    "1.1.0",
			want:     []string{"1.0.0", "1.1.0", "1.2.0"},
		},
		{
			name:     "empty slice returns empty slice",
			versions: []string{},
			since:    "1.0.0",
			want:     []string{},
		},
		{
			name:     "nil input returns nil",
			versions: nil,
			since:    "1.0.0",
			want:     nil,
		},
		{
			name:     "empty since string",
			versions: []string{"1.0.0", "2.0.0"},
			since:    "",
			want:     []string{"1.0.0", "2.0.0"},
		},
		{
			name:     "single version",
			versions: []string{"1.0.0"},
			since:    "0.5.0",
			want:     []string{"1.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterSince(tt.versions, tt.since)

			if !stringSliceEqual(got, tt.want) {
				t.Errorf("filterSince(%v, %q) = %v, want %v", tt.versions, tt.since, got, tt.want)
			}
		})
	}
}

func TestFilterRange_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		versions []string
		minVer   string
		maxVer   string
		want     []string
	}{
		{
			name:     "both bounds inclusive",
			versions: []string{"1.0.0", "1.1.0", "1.2.0", "1.3.0", "1.4.0"},
			minVer:   "1.1.0",
			maxVer:   "1.3.0",
			want:     []string{"1.1.0", "1.2.0", "1.3.0"},
		},
		{
			name:     "no bounds returns all",
			versions: []string{"1.0.0", "2.0.0", "3.0.0"},
			minVer:   "",
			maxVer:   "",
			want:     []string{"1.0.0", "2.0.0", "3.0.0"},
		},
		{
			name:     "empty input",
			versions: []string{},
			minVer:   "1.0.0",
			maxVer:   "2.0.0",
			want:     nil,
		},
		{
			name:     "nil input",
			versions: nil,
			minVer:   "1.0.0",
			maxVer:   "2.0.0",
			want:     nil,
		},
		{
			name:     "no versions in range",
			versions: []string{"1.0.0", "2.0.0", "3.0.0"},
			minVer:   "4.0.0",
			maxVer:   "5.0.0",
			want:     nil,
		},
		{
			name:     "exact match single version",
			versions: []string{"1.0.0", "2.0.0", "3.0.0"},
			minVer:   "2.0.0",
			maxVer:   "2.0.0",
			want:     []string{"2.0.0"},
		},
		{
			name:     "inverted range matches nothing",
			versions: []string{"1.0.0", "2.0.0", "3.0.0"},
			minVer:   "3.0.0",
			maxVer:   "1.0.0",
			want:     nil,
		},
		{
			name:     "patch-level filtering",
			versions: []string{"1.0.0", "1.0.1", "1.0.2", "1.0.3", "1.0.4"},
			minVer:   "1.0.1",
			maxVer:   "1.0.3",
			want:     []string{"1.0.1", "1.0.2", "1.0.3"},
		},
		{
			name:     "single element in range",
			versions: []string{"1.5.0"},
			minVer:   "1.0.0",
			maxVer:   "2.0.0",
			want:     []string{"1.5.0"},
		},
		{
			name:     "single element below range",
			versions: []string{"0.5.0"},
			minVer:   "1.0.0",
			maxVer:   "2.0.0",
			want:     nil,
		},
		{
			name:     "single element above range",
			versions: []string{"3.0.0"},
			minVer:   "1.0.0",
			maxVer:   "2.0.0",
			want:     nil,
		},
		{
			name:     "boundary: version equals min excluded if less than min",
			versions: []string{"0.9.9", "1.0.0", "1.0.1"},
			minVer:   "1.0.0",
			maxVer:   "",
			want:     []string{"1.0.0", "1.0.1"},
		},
		{
			name:     "boundary: version equals max included",
			versions: []string{"1.0.0", "1.0.1", "1.0.2"},
			minVer:   "",
			maxVer:   "1.0.1",
			want:     []string{"1.0.0", "1.0.1"},
		},
		{
			name:     "semantic comparison not lexicographic (1.10.0 > 1.2.0)",
			versions: []string{"1.2.0", "1.10.0"},
			minVer:   "1.3.0",
			maxVer:   "1.11.0",
			want:     []string{"1.10.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterRange(tt.versions, tt.minVer, tt.maxVer)

			if !stringSliceEqual(got, tt.want) {
				t.Errorf("filterRange(%v, %q, %q) = %v, want %v",
					tt.versions, tt.minVer, tt.maxVer, got, tt.want)
			}
		})
	}
}

// stringSliceEqual compares two string slices for equality.
// Treats nil and empty slices as distinct (nil != []string{}).
func stringSliceEqual(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
