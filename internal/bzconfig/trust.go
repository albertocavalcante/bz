package bzconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// EnvTrustedConfigDirs lists directories (separated like PATH) whose
// project config files are trusted without the ownership and permission
// checks, for legitimate shared setups such as a group-writable checkout.
// A workspace marker in a listed directory counts regardless of its owner.
//
// Trust applies to exactly the listed directories, compared by file
// identity after resolving symbolic links, never to their subdirectories.
// Every entry must be an absolute path. Listing a directory that others can
// write to trusts whatever they put there, which defeats the purpose of
// the checks.
const EnvTrustedConfigDirs = "BZ_TRUSTED_CONFIG_DIRS"

// UntrustedReason is the stable reason code of an [*UntrustedConfigError].
type UntrustedReason int

// Reasons a project config file is refused.
const (
	// ReasonNotRegular: the file (or its symlink target) is not a regular file.
	ReasonNotRegular UntrustedReason = iota + 1
	// ReasonOwnerUnknown: the platform does not expose the file's owner.
	ReasonOwnerUnknown
	// ReasonFileOwner: the file is owned by another user (not root).
	ReasonFileOwner
	// ReasonFileWritable: the file is writable by group or others.
	ReasonFileWritable
	// ReasonDirOwner: the containing directory is owned by another user.
	ReasonDirOwner
	// ReasonDirWritable: the containing directory is writable by group or
	// others and is not a sticky directory holding a file of the user's.
	ReasonDirWritable
)

func (r UntrustedReason) String() string {
	switch r {
	case ReasonNotRegular:
		return "not-regular"
	case ReasonOwnerUnknown:
		return "owner-unknown"
	case ReasonFileOwner:
		return "file-owner"
	case ReasonFileWritable:
		return "file-writable"
	case ReasonDirOwner:
		return "dir-owner"
	case ReasonDirWritable:
		return "dir-writable"
	default:
		return fmt.Sprintf("reason(%d)", int(r))
	}
}

// ErrUntrustedConfig matches every [*UntrustedConfigError] with errors.Is.
var ErrUntrustedConfig = errors.New("untrusted project config")

// UntrustedConfigError reports a project config file that fails the trust
// rules. Match it with errors.As, or with errors.Is(err, ErrUntrustedConfig).
type UntrustedConfigError struct {
	// Path is the file as found; Target is its resolved path (equal to
	// Path unless Path is a symbolic link); Dir is Target's directory.
	Path, Target, Dir string
	Reason            UntrustedReason
	// Detail describes the problem and Fix the concrete remedy.
	Detail, Fix string
}

func (e *UntrustedConfigError) Error() string {
	where := displayPath(e.Path)
	if e.Target != e.Path {
		where = fmt.Sprintf("%s (resolved to %s)", where, displayPath(e.Target))
	}
	return fmt.Sprintf("untrusted project config %s: %s; fix: %s, or list %s in %s if the shared setup is intended",
		where, e.Detail, e.Fix, displayPath(e.Dir), EnvTrustedConfigDirs)
}

// Is makes errors.Is(err, ErrUntrustedConfig) match.
func (e *UntrustedConfigError) Is(target error) bool { return target == ErrUntrustedConfig }

// CheckProjectConfigTrust refuses a project config file that another user
// could have planted or changed. A project file can redirect the registry
// or re-enable disabled commands, so bz must neither run under a file it
// cannot trust nor silently ignore it: an untrusted file is an
// [*UntrustedConfigError]. It also returns warning-level diagnostics for
// BZ_TRUSTED_CONFIG_DIRS entries that do not exist or cannot be resolved.
//
// The checks apply to the resolved target when path is a symbolic link:
//
//   - it must be a regular file;
//   - it must be owned by the current user or root;
//   - it must not be writable by group or others;
//   - its directory must be owned by the current user or root, always;
//   - its directory must not be writable by group or others, unless it
//     has the sticky bit set and the file is owned by the current user
//     (another user then cannot replace it). Your own file in root-owned
//     sticky /tmp is trusted; in a sticky directory owned by another
//     ordinary user it is refused, because that user can still replace it.
//
// Only the immediate directory is checked, not every ancestor. ACLs and
// extended attributes are not visible to these checks. A file in a
// directory listed in BZ_TRUSTED_CONFIG_DIRS skips the ownership and
// permission checks. On Windows, ownership and mode bits do not map to
// these rules and only the regular-file check applies.
//
// Every refusal suggests a fix. Commands are suggested only for files the
// current user owns, with each path shell-quoted after "--"; anything else
// is described (who owns it, and that an administrator must change it, or
// to keep the project in a directory you own).
func CheckProjectConfigTrust(e Environment, path string) ([]string, error) {
	if err := e.requireSearchFuncs(); err != nil {
		return nil, err
	}
	trusted, warnings, err := e.trustedDirs()
	if err != nil {
		return warnings, err
	}
	return warnings, e.checkTrust(path, trusted)
}

// checkTrust applies the trust rules with trusted already parsed.
func (e Environment) checkTrust(path string, trusted []fs.FileInfo) error {
	target, err := e.resolve(path)
	if err != nil {
		return fmt.Errorf("checking project config %s: %w", displayPath(path), cleanErr(err))
	}
	dir := filepath.Dir(target)
	refuse := func(reason UntrustedReason, detail, fix string) error {
		return &UntrustedConfigError{Path: path, Target: target, Dir: dir, Reason: reason, Detail: detail, Fix: fix}
	}

	fi, ok, err := e.stat(target)
	if err != nil {
		return err
	}
	if !ok || !fi.Mode().IsRegular() {
		return refuse(ReasonNotRegular, "not a regular file", "replace it with a regular file")
	}
	if e.GOOS == goosWindows {
		return nil
	}

	dfi, ok, err := e.stat(dir)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("checking project config %s: directory %s does not exist", displayPath(path), displayPath(dir))
	}
	inTrusted, err := e.isAnyOf(dir, trusted)
	if err != nil || inTrusted {
		return err
	}

	uid := e.Getuid()
	owner, known := e.FileOwner(fi)
	if !known {
		return refuse(ReasonOwnerUnknown, "its owner cannot be determined", "check its ownership")
	}
	if owner != uid && owner != 0 {
		return refuse(ReasonFileOwner,
			fmt.Sprintf("owned by uid %d, not you (uid %d) or root", owner, uid),
			fmt.Sprintf("recreate it as yourself, or have an administrator change its owner (it belongs to uid %d)", owner))
	}
	dirOwner, known := e.FileOwner(dfi)
	if !known {
		return refuse(ReasonOwnerUnknown, fmt.Sprintf("the owner of its directory %s cannot be determined", displayPath(dir)), "check the directory's ownership")
	}
	if dirOwner != uid && dirOwner != 0 {
		return refuse(ReasonDirOwner,
			fmt.Sprintf("its directory %s is owned by uid %d, not you (uid %d) or root, so that user can replace the file", displayPath(dir), dirOwner, uid),
			"keep the project in a directory you own")
	}

	return writableRefusal(refuse, fi, dfi, target, dir, uid, owner, dirOwner)
}

// writableRefusal applies the group/other-writable rules. It suggests
// chmod only for what the user owns and describes the rest.
func writableRefusal(refuse func(UntrustedReason, string, string) error, fi, dfi fs.FileInfo,
	target, dir string, uid, owner, dirOwner int,
) error {
	fileWritable := fi.Mode().Perm()&0o022 != 0
	stickyOwn := dfi.Mode()&fs.ModeSticky != 0 && owner == uid
	dirWritable := dfi.Mode().Perm()&0o022 != 0 && !stickyOwn

	// Suggest chmod only for what the user owns; describe the rest.
	var chmod, other []string
	if fileWritable {
		if owner == uid {
			chmod = append(chmod, target)
		} else {
			other = append(other, "the file is owned by root: have an administrator run chmod go-w on it, or recreate it as yourself")
		}
	}
	if dirWritable {
		if dirOwner == uid {
			chmod = append(chmod, dir)
		} else {
			other = append(other, "its directory is owned by root: keep the project in a directory you own")
		}
	}
	for _, p := range chmod {
		if !printable(p) {
			// Never offer a copyable command for a name with control or
			// non-printable characters.
			chmod = nil
			other = append([]string{"rename the file or directory to remove control or non-printable characters from its path, then fix its permissions"}, other...)
			break
		}
	}
	fix := strings.Join(other, "; ")
	if len(chmod) > 0 {
		cmd := "chmod go-w --"
		for _, p := range chmod {
			cmd += " " + shellQuote(p)
		}
		fix = strings.Join(append([]string{cmd}, other...), "; ")
	}

	switch {
	case fileWritable && dirWritable:
		return refuse(ReasonFileWritable,
			fmt.Sprintf("the file and its directory %s are writable by group or others (modes %s, %s)", displayPath(dir), fi.Mode().Perm(), dfi.Mode().Perm()),
			fix)
	case fileWritable:
		return refuse(ReasonFileWritable, fmt.Sprintf("writable by group or others (mode %s)", fi.Mode().Perm()), fix)
	case dirWritable:
		return refuse(ReasonDirWritable,
			fmt.Sprintf("its directory %s is writable by group or others (mode %s) and is not a sticky directory holding a file you own", displayPath(dir), dfi.Mode().Perm()),
			fix)
	}
	return nil
}

// shellQuote quotes s for a POSIX shell with single quotes, so a suggested
// command is safe to copy whatever the path contains.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// requireSearchFuncs fails closed when the environment lacks a function the
// search or trust rules need.
func (e Environment) requireSearchFuncs() error {
	var missing []string
	for _, f := range []struct {
		name string
		ok   bool
	}{
		{"Getwd", e.Getwd != nil},
		{"Stat", e.Stat != nil},
		{"EvalSymlinks", e.EvalSymlinks != nil},
		{"SameFile", e.SameFile != nil},
		{"Getuid", e.Getuid != nil || e.GOOS == goosWindows},
		{"FileOwner", e.FileOwner != nil || e.GOOS == goosWindows},
	} {
		if !f.ok {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("cannot search for a project config: the environment does not provide %s", strings.Join(missing, ", "))
	}
	return nil
}

// trustedDirs parses BZ_TRUSTED_CONFIG_DIRS into the FileInfos of the listed
// directories, for comparison by identity. A relative entry is an error; an
// entry that does not exist or cannot be resolved becomes a warning.
func (e Environment) trustedDirs() ([]fs.FileInfo, []string, error) {
	var (
		out      []fs.FileInfo
		warnings []string
	)
	for _, entry := range filepath.SplitList(e.getenv(EnvTrustedConfigDirs)) {
		if entry == "" {
			continue
		}
		if !filepath.IsAbs(entry) {
			return nil, warnings, fmt.Errorf("%s entry %q is not an absolute path; it would trust whatever directory bz runs in", EnvTrustedConfigDirs, entry)
		}
		resolved, err := e.resolve(entry)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s entry %q cannot be resolved (%v); ignored", EnvTrustedConfigDirs, entry, cleanErr(err)))
			continue
		}
		fi, ok, err := e.stat(resolved)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s entry %q cannot be checked (%v); ignored", EnvTrustedConfigDirs, entry, cleanErr(err)))
			continue
		}
		if !ok {
			warnings = append(warnings, fmt.Sprintf("%s entry %q does not exist; ignored", EnvTrustedConfigDirs, entry))
			continue
		}
		out = append(out, fi)
	}
	return out, warnings, nil
}

// resolve applies EvalSymlinks.
func (e Environment) resolve(path string) (string, error) {
	return e.EvalSymlinks(path)
}

// homeAndAncestors returns the home directory (resolved) and each of its
// ancestors, for the rule that none of them is a workspace root. It
// returns nil, turning the rule off, when there is no home directory or it
// cannot be resolved.
func (e Environment) homeAndAncestors() ([]fs.FileInfo, error) {
	home := e.home()
	if home == "" {
		return nil, nil
	}
	resolved, err := e.resolve(home)
	if err != nil {
		return nil, nil //nolint:nilerr // an unresolvable home turns the home rule off, as documented
	}
	var out []fs.FileInfo
	for dir := resolved; ; {
		fi, ok, err := e.stat(dir)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, fi)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return out, nil
		}
		dir = parent
	}
}

// isAnyOf reports whether dir is (by identity) one of infos.
func (e Environment) isAnyOf(dir string, infos []fs.FileInfo) (bool, error) {
	if len(infos) == 0 {
		return false, nil
	}
	fi, ok, err := e.stat(dir)
	if err != nil || !ok {
		return false, err
	}
	for _, h := range infos {
		if e.SameFile(fi, h) {
			return true, nil
		}
	}
	return false, nil
}
