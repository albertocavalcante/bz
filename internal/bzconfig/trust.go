package bzconfig

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

// EnvTrustedConfigDirs lists directories (separated like PATH) whose
// project config files are trusted without the ownership and permission
// checks, for legitimate shared setups such as a group-writable
// repository checkout.
const EnvTrustedConfigDirs = "BZ_TRUSTED_CONFIG_DIRS"

// CheckProjectConfigTrust refuses a project config file that another user
// could have planted or changed. A project file can redirect the registry
// or re-enable disabled commands, so bz must neither run under a file it
// cannot trust nor silently ignore it: an untrusted file is an error.
//
// The checks apply to the resolved target when path is a symbolic link:
//
//   - it must be a regular file;
//   - it must be owned by the current user or root;
//   - it must not be writable by group or others;
//   - its directory must not be writable by group or others, unless the
//     directory has the sticky bit set and the file is owned by the
//     current user (another user then cannot replace it).
//
// A file in a directory listed in BZ_TRUSTED_CONFIG_DIRS (compared by file
// identity) skips the ownership and permission checks. On Windows,
// ownership and mode bits do not map to these rules and only the
// regular-file check applies.
func CheckProjectConfigTrust(e Environment, path string) error {
	target, err := e.resolve(path)
	if err != nil {
		return fmt.Errorf("checking project config %s: %w", path, err)
	}
	fi, ok, err := e.stat(target)
	if err != nil {
		return err
	}
	if !ok || !fi.Mode().IsRegular() {
		return untrusted(path, target, "not a regular file")
	}
	if e.GOOS == "windows" {
		return nil
	}

	dir := filepath.Dir(target)
	dfi, ok, err := e.stat(dir)
	if err != nil {
		return err
	}
	if !ok {
		return untrusted(path, target, "its directory does not exist")
	}
	trusted, err := e.inTrustedDirs(dfi)
	if err != nil || trusted {
		return err
	}

	if e.Getuid == nil || e.FileOwner == nil {
		return untrusted(path, target, "its owner cannot be determined")
	}
	uid := e.Getuid()
	owner, ok := e.FileOwner(fi)
	if !ok {
		return untrusted(path, target, "its owner cannot be determined")
	}
	if owner != uid && owner != 0 {
		return untrusted(path, target, fmt.Sprintf("owned by uid %d, not the current user (uid %d) or root", owner, uid))
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return untrusted(path, target, fmt.Sprintf("writable by group or others (mode %s)", fi.Mode().Perm()))
	}
	if dfi.Mode().Perm()&0o022 != 0 {
		sticky := dfi.Mode()&fs.ModeSticky != 0
		if !sticky || owner != uid {
			return untrusted(path, target, fmt.Sprintf(
				"directory %s is writable by group or others (mode %s) and is not a sticky directory holding a file you own",
				dir, dfi.Mode().Perm()))
		}
	}
	return nil
}

func untrusted(path, target, reason string) error {
	where := path
	if target != path {
		where = fmt.Sprintf("%s (resolved to %s)", path, target)
	}
	return fmt.Errorf("untrusted project config %s: %s; fix its ownership or permissions, or list its directory in %s",
		where, reason, EnvTrustedConfigDirs)
}

// inTrustedDirs reports whether dir (by identity) is listed in
// BZ_TRUSTED_CONFIG_DIRS. Entries that do not exist are ignored.
func (e Environment) inTrustedDirs(dir fs.FileInfo) (bool, error) {
	if e.SameFile == nil {
		return false, nil
	}
	for _, entry := range filepath.SplitList(e.getenv(EnvTrustedConfigDirs)) {
		if entry == "" {
			continue
		}
		resolved, err := e.resolve(entry)
		if err != nil {
			continue // a listed directory that does not resolve trusts nothing
		}
		fi, ok, err := e.stat(resolved)
		if err != nil {
			return false, err
		}
		if ok && e.SameFile(dir, fi) {
			return true, nil
		}
	}
	return false, nil
}

// resolve applies EvalSymlinks when the environment provides it.
func (e Environment) resolve(path string) (string, error) {
	if e.EvalSymlinks == nil {
		return path, nil
	}
	return e.EvalSymlinks(path)
}

// homeAndAncestors returns the home directory (resolved) and each of its
// ancestors, for the rule that none of them is a workspace root. It
// returns nil when there is no home directory or no SameFile.
func (e Environment) homeAndAncestors() ([]fs.FileInfo, error) {
	home := e.home()
	if home == "" || e.SameFile == nil {
		return nil, nil
	}
	if resolved, err := e.resolve(home); err == nil {
		home = resolved
	}
	var out []fs.FileInfo
	for dir := home; ; {
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
