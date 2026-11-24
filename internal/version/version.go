// Package version provides semantic version parsing and comparison utilities.
package version

import (
	"runtime/debug"
	"strconv"
	"strings"
)

// UpdateType represents the type of version update.
type UpdateType int

const (
	// None indicates versions are equal.
	None UpdateType = iota
	// Patch indicates a patch version update (x.y.Z).
	Patch
	// Minor indicates a minor version update (x.Y.z).
	Minor
	// Major indicates a major version update (X.y.z).
	Major
	// Unknown indicates versions couldn't be compared as semver.
	Unknown
)

// String returns a human-readable representation of the update type.
func (u UpdateType) String() string {
	switch u {
	case None:
		return "-"
	case Patch:
		return "patch"
	case Minor:
		return "minor"
	case Major:
		return "major"
	default:
		return "unknown"
	}
}

// SemVer represents a parsed semantic version.
type SemVer struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Raw        string
}

// Parse attempts to parse a semantic version string.
// Returns nil if the string is not a valid semver.
func Parse(s string) *SemVer {
	if s == "" {
		return nil
	}

	// Remove leading 'v' if present
	s = strings.TrimPrefix(s, "v")

	// Split off prerelease/build metadata
	raw := s
	prerelease := ""
	if idx := strings.IndexAny(s, "-+"); idx != -1 {
		prerelease = s[idx:]
		s = s[:idx]
	}

	// Split into major.minor.patch
	parts := strings.Split(s, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return nil
	}

	// Parse major
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}

	// Parse minor (default to 0)
	minor := 0
	if len(parts) >= 2 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}
	}

	// Parse patch (default to 0)
	patch := 0
	if len(parts) >= 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil
		}
	}

	return &SemVer{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
		Raw:        raw,
	}
}

// Compare compares two SemVer versions.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func Compare(a, b *SemVer) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}

	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}

	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}

	// Compare prerelease: no prerelease > with prerelease
	// e.g., 1.0.0 > 1.0.0-rc1
	if a.Prerelease == "" && b.Prerelease != "" {
		return 1
	}
	if a.Prerelease != "" && b.Prerelease == "" {
		return -1
	}
	if a.Prerelease != b.Prerelease {
		if a.Prerelease < b.Prerelease {
			return -1
		}
		return 1
	}

	return 0
}

// CompareStrings compares two version strings.
// Falls back to string comparison if either is not valid semver.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareStrings(a, b string) int {
	semA := Parse(a)
	semB := Parse(b)

	if semA != nil && semB != nil {
		return Compare(semA, semB)
	}

	// Fallback to string comparison
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// ClassifyUpdate determines the type of update between two versions.
func ClassifyUpdate(current, latest string) UpdateType {
	if current == latest {
		return None
	}

	semCurrent := Parse(current)
	semLatest := Parse(latest)

	// If we can't parse as semver, it's unknown
	if semCurrent == nil || semLatest == nil {
		return Unknown
	}

	// If latest is not newer, no update
	if Compare(semCurrent, semLatest) >= 0 {
		return None
	}

	// Determine update type
	if semLatest.Major > semCurrent.Major {
		return Major
	}
	if semLatest.Minor > semCurrent.Minor {
		return Minor
	}
	if semLatest.Patch > semCurrent.Patch {
		return Patch
	}

	// Could be prerelease change
	return Patch
}

// Placeholder values that mean "not set by -ldflags".
const (
	// UnsetVersion is the default version string of a build without ldflags.
	UnsetVersion = "dev"
	// UnsetCommit is the default commit string of a build without ldflags.
	UnsetCommit = "none"
	// UnsetDate is the default build date of a build without ldflags.
	UnsetDate = "unknown"
)

const shortCommitLen = 7

// Info describes the running binary.
type Info struct {
	Version string
	Commit  string
	Date    string
}

// Resolve merges link-time values with the build metadata embedded by the Go
// toolchain. A link-time value (-ldflags -X) wins when it differs from its
// placeholder; otherwise the module version and vcs.* settings from bi are
// used, which is what `go install module@version` and `go build` in a VCS
// checkout provide. bi may be nil.
func Resolve(ldVersion, ldCommit, ldDate string, bi *debug.BuildInfo) Info {
	info := Info{Version: ldVersion, Commit: ldCommit, Date: ldDate}
	if bi == nil {
		return info
	}

	if isUnset(info.Version, UnsetVersion) {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			info.Version = v
		}
	}

	var revision, vcsTime string
	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if isUnset(info.Commit, UnsetCommit) && revision != "" {
		if len(revision) > shortCommitLen {
			revision = revision[:shortCommitLen]
		}
		if modified {
			revision += "-dirty"
		}
		info.Commit = revision
	}
	if isUnset(info.Date, UnsetDate) && vcsTime != "" {
		info.Date = vcsTime
	}
	return info
}

func isUnset(value, placeholder string) bool {
	return value == "" || value == placeholder
}

// Current resolves Info for the running binary from the given link-time values.
func Current(ldVersion, ldCommit, ldDate string) Info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		bi = nil
	}
	return Resolve(ldVersion, ldCommit, ldDate, bi)
}
