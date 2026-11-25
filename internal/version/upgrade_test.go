package version

import (
	"errors"
	"testing"

	"github.com/albertocavalcante/bz/internal/coord"
)

func TestRegistryUpgrade(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		current  string
		versions []string
		yanked   map[string]string
		latest   string
		want     UpdateType
	}{
		{"major", "1.9.9", []string{"2.0.0"}, nil, "2.0.0", Major},
		{"minor", "1.1.9", []string{"1.2.0"}, nil, "1.2.0", Minor},
		{"patch", "1.2.0", []string{"1.2.1"}, nil, "1.2.1", Patch},
		{"single segment major", "1", []string{"2"}, nil, "2", Major},
		{"two segment minor", "1.1", []string{"1.2"}, nil, "1.2", Minor},
		{"short release patch bump", "1.2", []string{"1.2.1"}, nil, "1.2.1", Patch},
		{"unordered stable", "1.0.0", []string{"3.0.0", "5.0.0", "9.0.0-next", "2.0.0"}, map[string]string{"5.0.0": ""}, "3.0.0", Major},
		{"numeric prerelease fallback", "1.0.0-rc.2", []string{"1.0.0-rc.10", "1.0.0-rc.9"}, nil, "1.0.0-rc.10", Patch},
		{"numeric prerelease downgrade", "1.0.0-rc.10", []string{"1.0.0-rc.9"}, nil, "1.0.0-rc.9", None},
		{"stable over same release prerelease", "1.0.0-preview", []string{"1.0.0", "1.0.0-preview"}, nil, "1.0.0", Patch},
		{"missing release segments", "1.2", []string{"1.2.0"}, nil, "1.2.0", NonSemVer},
		{"shorter release is lower", "1.2.0", []string{"1.2"}, nil, "1.2", None},
		{"BCR suffix upgrade", "1.2.3.bcr.9", []string{"1.2.3.bcr.10", "1.2.3.bcr.8"}, nil, "1.2.3.bcr.10", NonSemVer},
		{"BCR suffix downgrade", "1.2.3.bcr.10", []string{"1.2.3.bcr.9"}, nil, "1.2.3.bcr.9", None},
		{"text release segment", "2025", []string{"2025.snapshot"}, nil, "2025.snapshot", NonSemVer},
		{"numeric spelling breaks ties", "1.01.0", []string{"1.1.0"}, nil, "1.1.0", NonSemVer},
		{"unsigned numeric identifiers", "1.9223372036854775808.0", []string{"1.18446744073709551615.0"}, nil, "1.18446744073709551615.0", Minor},
		{"build metadata equality", "1.1.0+previous", []string{"1.1.0+other"}, nil, "1.1.0+other", None},
		{"build metadata is stable", "1.1.0", []string{"1.2.0+build-rc", "1.3.0-next"}, nil, "1.2.0+build-rc", Minor},
		{"prefer plain equivalent version", "1.1.0", []string{"1.2.0+build", "1.2.0"}, nil, "1.2.0", Minor},
		{"yank applies to build spellings", "1.1.0", []string{"2.0.0+build", "1.5.0"}, map[string]string{"2.0.0": "withdrawn"}, "1.5.0", Minor},
		{"yanked build applies to other spellings", "1.1.0", []string{"2.0.0+other", "2.0.0", "1.5.0"}, map[string]string{"2.0.0+build": "withdrawn"}, "1.5.0", Minor},
		{"Bazel ignores build segments", "1.1.0", []string{"1.2.0+foo..bar"}, nil, "1.2.0+foo..bar", NonSemVer},
		{"ignore malformed yanked", "1.0.0", []string{"1..0", "1.0.1"}, map[string]string{"1..0": ""}, "1.0.1", Patch},
		{"all yanked", "1.5.0", []string{"2.0.0"}, map[string]string{"2.0.0": ""}, "1.5.0", None},
		{"no candidates", "1.5.0", nil, nil, "1.5.0", None},
		{"malformed current", "1..0", []string{"1.2.0"}, nil, "1.2.0", Unknown},
		{"versionless", "", []string{"1.2.0"}, nil, "", None},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			latest, got, err := RegistryUpgrade(tt.current, tt.versions, tt.yanked)
			if err != nil || latest != tt.latest || got != tt.want {
				t.Errorf("RegistryUpgrade(%q, %v) = (%q, %v, %v), want (%q, %v, nil)", tt.current, tt.versions, latest, got, err, tt.latest, tt.want)
			}
		})
	}
}

func TestRegistryUpgradeRefusesMalformedCandidates(t *testing.T) {
	t.Parallel()
	for _, candidate := range []string{"", "1..0", "1.0.0-rc..2", "1.0.0-18446744073709551616", "1.0.0/path"} {
		t.Run(candidate, func(t *testing.T) {
			t.Parallel()
			latest, got, err := RegistryUpgrade("1.0.0", []string{"3.0.0", candidate, "2.0.0"}, nil)
			if !errors.Is(err, coord.ErrInvalidVersion) || latest != "1.0.0" || got != None {
				t.Errorf("RegistryUpgrade with invalid %q = (%q, %v, %v), want no update and invalid version", candidate, latest, got, err)
			}
		})
	}
}

func TestRegistryUpgradeRefusesUnsupportedTarget(t *testing.T) {
	t.Parallel()
	latest, got, err := RegistryUpgrade("2025", []string{"2026", "snapshot"}, nil)
	if !errors.Is(err, ErrUnsupportedBazelVersion) || latest != "snapshot" || got != None {
		t.Errorf("RegistryUpgrade with unsupported Bazel version = (%q, %v, %v)", latest, got, err)
	}
}
