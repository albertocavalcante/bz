// Package version provides version parsing, comparison, and update selection.
package version

import (
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/albertocavalcante/go-bzlmod-ast/label"
	bazelversion "github.com/albertocavalcante/go-bzlmod/selection/version"

	"github.com/albertocavalcante/bz/internal/coord"
)

// UpdateType represents the type of version update.
type UpdateType int

const (
	// None indicates no newer version is available.
	None UpdateType = iota
	// Patch indicates a patch version update (x.y.Z).
	Patch
	// Minor indicates a minor version update (x.Y.z).
	Minor
	// Major indicates a major version update (X.y.z).
	Major
	// Unknown indicates the current version couldn't be compared.
	Unknown
	// NonSemVer indicates a verified Bazel upgrade without a SemVer bump type.
	NonSemVer
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
	case NonSemVer:
		return "non-semver"
	default:
		return "unknown"
	}
}

// IsUpgrade reports whether the comparison found a newer version.
func (u UpdateType) IsUpgrade() bool {
	switch u {
	case Patch, Minor, Major, NonSemVer:
		return true
	default:
		return false
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

// ErrUnsupportedBazelVersion reports a valid Bazel upgrade that bz cannot read
// back from MODULE.bazel with its current parser.
var ErrUnsupportedBazelVersion = errors.New("bz MODULE.bazel parser does not support this Bazel version")

// RegistryUpgrade compares a dependency against the highest non-yanked stable
// candidate, or the highest prerelease if there is no stable candidate.
// Invalid unyanked registry entries produce an error because they cannot be
// ranked safely. An unparseable current version makes the upgrade unverified.
// A newer candidate must also be accepted by the MODULE.bazel parser.
func RegistryUpgrade(current string, versions []string, yanked map[string]string) (string, UpdateType, error) {
	if current == "" {
		return current, None, nil
	}

	latest, candidate, err := selectRegistryVersion(versions, yanked)
	if err != nil {
		return current, None, err
	}
	if latest == "" {
		return current, None, nil
	}

	if installed, err := bazelversion.Parse(current); err == nil {
		// Both versions parsed successfully, so Compare uses Bazel order.
		if bazelversion.Compare(latest, current) <= 0 {
			return latest, None, nil
		}
		if _, err := label.NewVersion(latest); err != nil {
			return latest, None, fmt.Errorf("%w: %q", ErrUnsupportedBazelVersion, latest)
		}
		return latest, classifyBump(current, latest, installed, candidate), nil
	}
	return latest, Unknown, nil
}

func selectRegistryVersion(versions []string, yanked map[string]string) (string, bazelversion.ParsedVersion, error) {
	// Bazel parses yanked version keys, so equivalent build spellings are
	// yanked together.
	yankedVersions := make(map[string]bool)
	for raw := range yanked {
		// Build metadata is ignored for identity. Only a prefix that fits a
		// registry coordinate can match a candidate.
		prefix, _, _ := strings.Cut(raw, "+")
		if len(prefix) > coord.MaxCoordinateLen {
			continue
		}
		if parsed, err := bazelversion.Parse(raw); err == nil {
			yankedVersions[parsed.Normalized] = true
		}
	}
	var latest string
	var selected bazelversion.ParsedVersion
	for _, candidate := range versions {
		if _, skip := yanked[candidate]; skip {
			continue
		}
		if err := coord.ValidateVersion(candidate); err != nil {
			return "", bazelversion.ParsedVersion{}, err
		}
		parsed, err := bazelversion.Parse(candidate)
		if err != nil {
			return "", bazelversion.ParsedVersion{}, err
		}
		if yankedVersions[parsed.Normalized] {
			continue
		}
		if latest == "" || preferRegistryVersion(candidate, latest, parsed, selected) {
			latest, selected = candidate, parsed
		}
	}
	return latest, selected, nil
}

func preferRegistryVersion(candidate, latest string, parsed, selected bazelversion.ParsedVersion) bool {
	candidateStable, latestStable := len(parsed.Prerelease) == 0, len(selected.Prerelease) == 0
	if candidateStable != latestStable {
		return candidateStable
	}
	order := bazelversion.Compare(candidate, latest)
	// Equivalent build variants use spelling as a deterministic tiebreaker.
	// If present, the version without build metadata comes first.
	return order > 0 || (order == 0 && candidate < latest)
}

func classifyBump(current, latest string, installed, candidate bazelversion.ParsedVersion) UpdateType {
	from, fromOK := conventionalRelease(current, installed)
	to, toOK := conventionalRelease(latest, candidate)
	if !fromOK || !toOK {
		return NonSemVer
	}
	switch {
	case to[0] != from[0]:
		return Major
	case to[1] != from[1]:
		return Minor
	case to[2] != from[2] || len(candidate.Release) == len(installed.Release):
		// A newer prerelease spelling of the same release is a patch update.
		return Patch
	default:
		// Appending zero-valued segments advances Bazel's ordering without
		// changing a conventional major, minor, or patch number.
		return NonSemVer
	}
}

// conventionalRelease accepts numeric release shorthand for bump labeling.
// Missing segments are zero here and have already been compared by Bazel.
func conventionalRelease(raw string, parsed bazelversion.ParsedVersion) ([3]uint64, bool) {
	var release [3]uint64
	if len(parsed.Release) > len(release) {
		return release, false
	}
	values := release[:0]
	for _, id := range parsed.Release {
		if !id.IsDigitsOnly || (len(id.AsString) > 1 && id.AsString[0] == '0') {
			return release, false
		}
		values = append(values, id.AsNumber)
	}
	for _, id := range parsed.Prerelease {
		if id.IsDigitsOnly && len(id.AsString) > 1 && id.AsString[0] == '0' {
			return release, false
		}
	}
	if _, build, ok := strings.Cut(raw, "+"); ok {
		for id := range strings.SplitSeq(build, ".") {
			if id == "" {
				return release, false
			}
		}
	}
	return release, true
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
// checkout provide. A leading "v" is dropped from the version either way.
// bi may be nil.
func Resolve(ldVersion, ldCommit, ldDate string, bi *debug.BuildInfo) Info {
	info := resolve(ldVersion, ldCommit, ldDate, bi)
	info.Version = trimV(info.Version)
	return info
}

// trimV drops the leading "v" of a semver-looking version ("v1.2.3" becomes
// "1.2.3") so tagged releases and go install builds display the same form.
func trimV(v string) string {
	if len(v) > 1 && v[0] == 'v' && v[1] >= '0' && v[1] <= '9' {
		return v[1:]
	}
	return v
}

func resolve(ldVersion, ldCommit, ldDate string, bi *debug.BuildInfo) Info {
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
