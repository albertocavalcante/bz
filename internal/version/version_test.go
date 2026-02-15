package version

import (
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  *SemVer
	}{
		{"1.0.0", &SemVer{Major: 1, Minor: 0, Patch: 0, Raw: "1.0.0"}},
		{"0.50.1", &SemVer{Major: 0, Minor: 50, Patch: 1, Raw: "0.50.1"}},
		{"2.3.4", &SemVer{Major: 2, Minor: 3, Patch: 4, Raw: "2.3.4"}},
		{"1.0", &SemVer{Major: 1, Minor: 0, Patch: 0, Raw: "1.0"}},
		{"1", &SemVer{Major: 1, Minor: 0, Patch: 0, Raw: "1"}},
		{"v1.0.0", &SemVer{Major: 1, Minor: 0, Patch: 0, Raw: "1.0.0"}},
		{"1.0.0-rc1", &SemVer{Major: 1, Minor: 0, Patch: 0, Prerelease: "-rc1", Raw: "1.0.0-rc1"}},
		{"1.0.0-alpha.1", &SemVer{Major: 1, Minor: 0, Patch: 0, Prerelease: "-alpha.1", Raw: "1.0.0-alpha.1"}},
		{"1.0.0+build", &SemVer{Major: 1, Minor: 0, Patch: 0, Prerelease: "+build", Raw: "1.0.0+build"}},
		{"", nil},
		{"invalid", nil},
		{"a.b.c", nil},
		{"1.2.3.4", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := Parse(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("Parse(%q) = %+v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Errorf("Parse(%q) = nil, want %+v", tt.input, tt.want)
				return
			}
			if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch {
				t.Errorf("Parse(%q) = %d.%d.%d, want %d.%d.%d",
					tt.input, got.Major, got.Minor, got.Patch,
					tt.want.Major, tt.want.Minor, tt.want.Patch)
			}
			if got.Prerelease != tt.want.Prerelease {
				t.Errorf("Parse(%q).Prerelease = %q, want %q", tt.input, got.Prerelease, tt.want.Prerelease)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b string
		want int
	}{
		// Equal
		{"1.0.0", "1.0.0", 0},
		{"0.50.1", "0.50.1", 0},

		// Major differences
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},

		// Minor differences
		{"1.2.0", "1.1.0", 1},
		{"1.1.0", "1.2.0", -1},

		// Patch differences
		{"1.0.2", "1.0.1", 1},
		{"1.0.1", "1.0.2", -1},

		// Prerelease comparisons
		{"1.0.0", "1.0.0-rc1", 1},         // release > prerelease
		{"1.0.0-rc1", "1.0.0", -1},        // prerelease < release
		{"1.0.0-rc2", "1.0.0-rc1", 1},     // rc2 > rc1 (string comparison)
		{"1.0.0-alpha", "1.0.0-beta", -1}, // alpha < beta
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			t.Parallel()
			semA := Parse(tt.a)
			semB := Parse(tt.b)
			if semA == nil || semB == nil {
				t.Fatalf("Failed to parse versions: %q, %q", tt.a, tt.b)
			}
			got := Compare(semA, semB)
			if got != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCompareStrings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b string
		want int
	}{
		// Valid semver
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},

		// Fallback to string comparison
		{"abc", "def", -1},
		{"def", "abc", 1},
		{"same", "same", 0},

		// Mixed (one valid, one invalid) - falls back to string comparison
		{"1.0.0", "invalid", -1}, // "1.0.0" < "invalid" (string comparison)
		{"invalid", "1.0.0", 1},  // "invalid" > "1.0.0" (string comparison)
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			t.Parallel()
			got := CompareStrings(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareStrings(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestClassifyUpdate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		current, latest string
		want            UpdateType
	}{
		// No update
		{"1.0.0", "1.0.0", None},
		{"0.50.1", "0.50.1", None},

		// Patch update
		{"1.0.0", "1.0.1", Patch},
		{"1.0.5", "1.0.10", Patch},

		// Minor update
		{"1.0.0", "1.1.0", Minor},
		{"1.5.0", "1.10.0", Minor},
		{"1.0.5", "1.1.0", Minor},

		// Major update
		{"1.0.0", "2.0.0", Major},
		{"0.50.1", "1.0.0", Major},
		{"1.5.3", "2.0.0", Major},

		// Current is newer (no update)
		{"2.0.0", "1.0.0", None},
		{"1.1.0", "1.0.0", None},

		// Invalid versions (unknown)
		{"invalid", "1.0.0", Unknown},
		{"1.0.0", "invalid", Unknown},
		{"abc", "def", Unknown},

		// Prerelease to release
		{"1.0.0-rc1", "1.0.0", Patch},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_to_"+tt.latest, func(t *testing.T) {
			t.Parallel()
			got := ClassifyUpdate(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("ClassifyUpdate(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestUpdateTypeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ut   UpdateType
		want string
	}{
		{None, "-"},
		{Patch, "patch"},
		{Minor, "minor"},
		{Major, "major"},
		{Unknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.ut.String(); got != tt.want {
				t.Errorf("UpdateType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
