// Package coord validates Bazel module coordinates (module names and
// versions) before they are used as path or URL segments.
//
// Module names and versions reach the filesystem and network from untrusted
// sources: command-line arguments, a project's MODULE.bazel, and registry
// metadata. Every place that turns a coordinate into a path segment must call
// [ValidateModuleName] and [ValidateVersion] (or [Validate]) first, and should
// then confirm the result with [Join] or [Within].
//
// The rules follow Bazel's own definitions:
//
//   - Module names: RepositoryName.VALID_MODULE_NAME,
//     "[a-z]([a-z0-9._-]*[a-z0-9])?".
//   - Versions: bzlmod Version.parse, RELEASE[-PRERELEASE][+BUILD] where
//     RELEASE is "[a-zA-Z0-9.]+", PRERELEASE is "[a-zA-Z0-9.-]+", BUILD is
//     "[a-zA-Z0-9.-]+", and RELEASE and PRERELEASE are dot-separated
//     non-empty identifiers.
//
// Bazel also accepts the empty version for modules with a non-registry
// override. An empty version never names a registry directory, so
// [ValidateVersion] rejects it: callers must not build a path from it.
//
// Names and versions are additionally capped at [MaxCoordinateLen] bytes so
// a single segment stays within common filesystem name limits.
//
// Some coordinates Bazel accepts are awkward on particular filesystems, and
// they are accepted here too because rejecting them would reject valid
// modules: Windows device names such as "con" or "aux" are valid module
// names, and versions that differ only in case (for example "1.0.A" and
// "1.0.a") collide on case-insensitive filesystems. Neither can address a
// location outside the base directory.
package coord

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Sentinel errors; returned errors wrap one of these.
var (
	// ErrInvalidModuleName reports a module name that does not follow Bazel's rule.
	ErrInvalidModuleName = errors.New("invalid module name")

	// ErrInvalidVersion reports a version that does not follow Bazel's syntax.
	ErrInvalidVersion = errors.New("invalid module version")

	// ErrInvalidPath reports a relative path that is not a safe registry path.
	ErrInvalidPath = errors.New("invalid registry path")

	// ErrEscapesBase reports a path that resolves outside its base directory.
	ErrEscapesBase = errors.New("path escapes base directory")
)

// Length limits, in bytes.
const (
	// MaxCoordinateLen caps a module name or version.
	MaxCoordinateLen = 255
	// MaxSegmentLen caps any segment of a registry path.
	MaxSegmentLen = 255
	// MaxRegistryPathLen caps a whole registry path.
	MaxRegistryPathLen = 1024
)

// fileSegmentPattern restricts registry path segments below the version
// directory (file names such as "MODULE.bazel" or "patches/fix.patch"): no
// leading dot, no separators, and none of '%', '?', '#', ':', whitespace or
// control characters.
var fileSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9_+-][A-Za-z0-9._+-]*$`)

// moduleNamePattern is Bazel's RepositoryName.VALID_MODULE_NAME.
var moduleNamePattern = regexp.MustCompile(`^[a-z]([a-z0-9._-]*[a-z0-9])?$`)

// versionPattern is Bazel's bzlmod Version.PATTERN.
var versionPattern = regexp.MustCompile(`^([a-zA-Z0-9.]+)(?:-([a-zA-Z0-9.-]+))?(?:\+[a-zA-Z0-9.-]+)?$`)

// ValidateModuleName reports whether name is a valid Bazel module name.
func ValidateModuleName(name string) error {
	if len(name) > MaxCoordinateLen {
		return fmt.Errorf("%w: name is %d bytes, longer than the %d-byte limit", ErrInvalidModuleName, len(name), MaxCoordinateLen)
	}
	if !moduleNamePattern.MatchString(name) {
		return fmt.Errorf("%w %q: must start with a lowercase letter, end with a lowercase letter or digit, "+
			"and contain only a-z, 0-9, '.', '-' and '_'", ErrInvalidModuleName, name)
	}
	return nil
}

// ValidateVersion reports whether version is a non-empty valid Bazel module
// version. The empty version (used by Bazel for overridden modules) is
// rejected because it cannot name a registry directory.
func ValidateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("%w: version is empty (overridden modules have no registry version)", ErrInvalidVersion)
	}
	if len(version) > MaxCoordinateLen {
		return fmt.Errorf("%w: version is %d bytes, longer than the %d-byte limit", ErrInvalidVersion, len(version), MaxCoordinateLen)
	}
	m := versionPattern.FindStringSubmatch(version)
	if m == nil {
		return fmt.Errorf("%w %q: must match RELEASE[-PRERELEASE][+BUILD] using only A-Z, a-z, 0-9, '.', '-' and '+'",
			ErrInvalidVersion, version)
	}
	if err := validateIdentifiers(m[1]); err != nil {
		return fmt.Errorf("%w %q: release %w", ErrInvalidVersion, version, err)
	}
	if m[2] != "" {
		if err := validateIdentifiers(m[2]); err != nil {
			return fmt.Errorf("%w %q: prerelease %w", ErrInvalidVersion, version, err)
		}
	}
	return nil
}

// validateIdentifiers mirrors Version.Identifier.from: every dot-separated
// identifier is non-empty and numeric identifiers fit in an unsigned 64-bit
// integer. Current Bazel (8.4 and later) parses numeric identifiers with
// Long.parseUnsignedLong; Bazel 7.x and 8.0 used Integer.parseInt, so values
// above 2147483647 only work with newer Bazel. The bound is not a safety
// property: digits can never form a path traversal.
func validateIdentifiers(s string) error {
	for ident := range strings.SplitSeq(s, ".") {
		if ident == "" {
			return errors.New("has an empty identifier")
		}
		if isDigits(ident) {
			if _, err := strconv.ParseUint(ident, 10, 64); err != nil {
				return fmt.Errorf("numeric identifier %q is too large", ident)
			}
		}
	}
	return nil
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Validate validates a module name and version together.
func Validate(name, version string) error {
	if err := ValidateModuleName(name); err != nil {
		return err
	}
	return ValidateVersion(version)
}

// ValidateRegistryPath validates a slash-separated path relative to a
// registry's modules directory, such as "rules_go/0.50.0/MODULE.bazel",
// "rules_go/metadata.json" or "rules_go/0.50.0/patches/fix.patch".
//
// The first segment must be a valid module name. With three or more
// segments the second must be a valid version. Every later (file) segment
// must match [A-Za-z0-9_+-][A-Za-z0-9._+-]*, so it has no leading dot and
// none of '%', '?', '#', ':', whitespace or control characters. No segment
// may be ".git" in any letter case, segments are capped at [MaxSegmentLen]
// bytes and the whole path at [MaxRegistryPathLen] bytes.
func ValidateRegistryPath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") || filepath.IsAbs(p) {
		return fmt.Errorf("%w %q", ErrInvalidPath, p)
	}
	if len(p) > MaxRegistryPathLen {
		return fmt.Errorf("%w: path is %d bytes, longer than the %d-byte limit", ErrInvalidPath, len(p), MaxRegistryPathLen)
	}
	segs := strings.Split(p, "/")
	if len(segs) < 2 {
		return fmt.Errorf("%w %q: want <module>/<file> or <module>/<version>/<file>", ErrInvalidPath, p)
	}
	for _, s := range segs {
		if s == "" || s == "." || s == ".." {
			return fmt.Errorf("%w %q: empty, '.' or '..' segment", ErrInvalidPath, p)
		}
		if strings.EqualFold(s, ".git") {
			return fmt.Errorf("%w %q: '.git' segment", ErrInvalidPath, p)
		}
		if len(s) > MaxSegmentLen {
			return fmt.Errorf("%w %q: segment longer than the %d-byte limit", ErrInvalidPath, p, MaxSegmentLen)
		}
	}
	if err := ValidateModuleName(segs[0]); err != nil {
		return fmt.Errorf("%w %q: %w", ErrInvalidPath, p, err)
	}
	files := segs[1:]
	if len(segs) >= 3 {
		if err := ValidateVersion(segs[1]); err != nil {
			return fmt.Errorf("%w %q: %w", ErrInvalidPath, p, err)
		}
		files = segs[2:]
	}
	for _, f := range files {
		if !fileSegmentPattern.MatchString(f) {
			return fmt.Errorf("%w %q: file segment %q must match [A-Za-z0-9_+-][A-Za-z0-9._+-]*", ErrInvalidPath, p, f)
		}
	}
	return nil
}

// Within reports an error unless target is base itself or lies lexically
// inside base. Both paths are cleaned and made absolute first. The check is
// purely lexical: it does not resolve symbolic links that already exist
// inside base.
func Within(base, target string) error {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("resolve base: %w", err)
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return fmt.Errorf("%w: %q is not under %q", ErrEscapesBase, target, base)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("%w: %q is not under %q", ErrEscapesBase, target, base)
	}
	return nil
}

// Join joins elems onto base and verifies that the result lies strictly
// inside base (not base itself). Use it after validating the coordinates
// that make up elems; it is a second layer, not a substitute.
func Join(base string, elems ...string) (string, error) {
	joined := filepath.Join(append([]string{base}, elems...)...)
	if err := Within(base, joined); err != nil {
		return "", err
	}
	if filepath.Clean(joined) == filepath.Clean(base) {
		return "", fmt.Errorf("%w: %q resolves to the base directory itself", ErrEscapesBase, filepath.Join(elems...))
	}
	return joined, nil
}
