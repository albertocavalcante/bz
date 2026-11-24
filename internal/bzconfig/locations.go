package bzconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Project configuration file names, searched in each directory. ".bz.toml"
// is the documented name; ".bzconfig.toml" is the name earlier releases
// read and is still accepted. Both in one directory is an error.
const (
	ProjectConfigName       = ".bz.toml"
	LegacyProjectConfigName = ".bzconfig.toml"
)

// goosWindows is the GOOS value that selects Windows locations and rules.
const goosWindows = "windows"

// workspaceMarkers identify a Bazel workspace root; the project file search
// stops there.
var workspaceMarkers = []string{"MODULE.bazel", "REPO.bazel", "WORKSPACE.bazel", "WORKSPACE"}

// Environment is everything configuration loading reads from the outside
// world. Tests inject a fake one so they never touch the real home
// directory, /etc or process environment.
type Environment struct {
	// GOOS selects platform-specific locations ("windows" or anything else).
	GOOS        string
	LookupEnv   func(string) (string, bool)
	Getwd       func() (string, error)
	UserHomeDir func() (string, error)
	ReadFile    func(string) ([]byte, error)
	Stat        func(string) (fs.FileInfo, error)
	// EvalSymlinks resolves symbolic links (filepath.EvalSymlinks); nil
	// means paths are used as given.
	EvalSymlinks func(string) (string, error)
	// SameFile reports whether two FileInfos describe the same file
	// (os.SameFile); nil disables the home-directory rule.
	SameFile func(a, b fs.FileInfo) bool
	// Getuid returns the current user id (os.Getuid).
	Getuid func() int
	// FileOwner returns the owner uid of a file, and false when the
	// platform does not expose one.
	FileOwner func(fs.FileInfo) (int, bool)
	// SystemConfigPath is the system-wide file; empty means none.
	SystemConfigPath string
}

// OSEnvironment returns the real process environment.
func OSEnvironment() Environment {
	env := Environment{
		GOOS:        runtime.GOOS,
		LookupEnv:   os.LookupEnv,
		Getwd:       os.Getwd,
		UserHomeDir: os.UserHomeDir,
		ReadFile:    os.ReadFile,
		Stat:        os.Stat,

		EvalSymlinks: filepath.EvalSymlinks,
		SameFile:     os.SameFile,
		Getuid:       os.Getuid,
		FileOwner:    fileOwner,
	}
	env.SystemConfigPath = SystemConfigPath(env.GOOS, env.LookupEnv)
	return env
}

func (e Environment) getenv(name string) string {
	if e.LookupEnv == nil {
		return ""
	}
	v, _ := e.LookupEnv(name)
	return v
}

func (e Environment) home() string {
	if e.UserHomeDir == nil {
		return ""
	}
	h, err := e.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// stat returns path's FileInfo and whether it exists, and an error naming
// path for any failure other than "not exist" (permission denied, I/O
// error): only "not exist" means absent.
func (e Environment) stat(path string) (fs.FileInfo, bool, error) {
	if e.Stat == nil {
		return nil, false, nil
	}
	fi, err := e.Stat(path)
	switch {
	case err == nil:
		return fi, true, nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("checking %s: %w", path, err)
	}
}

// regularFile reports whether path exists as a regular file. A path that
// exists but is not a regular file (a directory, FIFO or device) is an
// error naming it.
func (e Environment) regularFile(path string) (bool, error) {
	fi, ok, err := e.stat(path)
	if err != nil || !ok {
		return false, err
	}
	if !fi.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular file (%s)", path, fi.Mode().Type())
	}
	return true, nil
}

// SystemConfigPath returns the system-wide config file:
// /etc/bz/config.toml, or %ProgramData%\bz\config.toml on Windows (none
// when ProgramData is unset).
func SystemConfigPath(goos string, lookup func(string) (string, bool)) string {
	if goos == goosWindows {
		if lookup == nil {
			return ""
		}
		pd, _ := lookup("ProgramData")
		if pd == "" {
			return ""
		}
		return filepath.Join(pd, "bz", "config.toml")
	}
	return "/etc/bz/config.toml"
}

// UserConfigPath returns the per-user config file:
//
//  1. $XDG_CONFIG_HOME/bz/config.toml when XDG_CONFIG_HOME is an absolute path;
//  2. on Windows, %AppData%\bz\config.toml;
//  3. otherwise ~/.config/bz/config.toml (on macOS too, as documented).
//
// It returns "" when no location can be determined.
func UserConfigPath(e Environment) string {
	if xdg := e.getenv("XDG_CONFIG_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "bz", "config.toml")
	}
	if e.GOOS == goosWindows {
		if appData := e.getenv("AppData"); appData != "" {
			return filepath.Join(appData, "bz", "config.toml")
		}
	}
	if home := e.home(); home != "" {
		return filepath.Join(home, ".config", "bz", "config.toml")
	}
	return ""
}

// DefaultCacheDir returns the built-in cache directory:
//
//  1. ~/.cache/bz when that directory already exists (the location every
//     earlier release used, so existing caches are never orphaned); when it
//     cannot be checked (permission denied, I/O error) it is kept too, so the
//     command that uses it reports the real error instead of silently
//     switching to another cache;
//  2. $XDG_CACHE_HOME/bz when XDG_CACHE_HOME is an absolute path;
//  3. on Windows, %LocalAppData%\bz\cache;
//  4. otherwise ~/.cache/bz.
func DefaultCacheDir(e Environment) string {
	home := e.home()
	legacy := ""
	if home != "" {
		legacy = filepath.Join(home, ".cache", "bz")
		if fi, ok, err := e.stat(legacy); err != nil || (ok && fi.IsDir()) {
			return legacy
		}
	}
	if xdg := e.getenv("XDG_CACHE_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "bz")
	}
	if e.GOOS == goosWindows {
		if local := e.getenv("LocalAppData"); local != "" {
			return filepath.Join(local, "bz", "cache")
		}
	}
	if legacy != "" {
		return legacy
	}
	return filepath.Join(".cache", "bz")
}

// FindProjectConfig searches for the project config file. It returns the
// path ("" when there is none), warning-level diagnostics for the caller to
// print (unusable BZ_TRUSTED_CONFIG_DIRS entries), and an error.
//
// When the working directory is inside a Bazel workspace (a directory
// holding a MODULE.bazel, REPO.bazel, WORKSPACE.bazel or WORKSPACE file),
// every directory from the working directory up to and including the
// workspace root is searched and the nearest file wins. The innermost
// marker is the root, so a nested workspace ends the search. Outside a
// workspace only the working directory is searched.
//
// Rules that keep other users from steering the search:
//
//   - The search follows the physical path: the working directory is
//     resolved with EvalSymlinks first.
//   - The home directory and its ancestors are never a workspace root
//     (compared by file identity); reaching one ends the walk as "outside
//     a workspace". A home directory located inside a workspace therefore
//     makes that workspace's marker ignored. When home is unset or cannot
//     be resolved, this rule is off.
//   - On platforms with file ownership (not Windows), a marker not owned by
//     the current user or root is skipped and the walk continues upward,
//     so a marker planted in a shared directory cannot cut the search
//     short or point it at a planted config.
//   - A file that is found must pass [CheckProjectConfigTrust]; an
//     untrusted file is an [*UntrustedConfigError].
//
// A marker or config path that exists but cannot be checked is an error
// naming it; only a path that does not exist counts as absent. An
// environment missing any function the rules need (Getwd, Stat,
// EvalSymlinks, SameFile, and Getuid and FileOwner outside Windows) is an
// error, never a silently weaker search.
func FindProjectConfig(e Environment) (string, []string, error) {
	if err := e.requireSearchFuncs(); err != nil {
		return "", nil, err
	}
	wd, err := e.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("finding project config: %w", err)
	}
	if wd, err = e.resolve(wd); err != nil {
		return "", nil, fmt.Errorf("finding project config: %w", err)
	}
	homeDirs, err := e.homeAndAncestors()
	if err != nil {
		return "", nil, err
	}
	dirs, err := projectSearchDirs(e, wd, homeDirs)
	if err != nil {
		return "", nil, err
	}
	for _, dir := range dirs {
		primary := filepath.Join(dir, ProjectConfigName)
		legacy := filepath.Join(dir, LegacyProjectConfigName)
		hasPrimary, err := e.regularFile(primary)
		if err != nil {
			return "", nil, err
		}
		hasLegacy, err := e.regularFile(legacy)
		if err != nil {
			return "", nil, err
		}
		switch {
		case hasPrimary && hasLegacy:
			return "", nil, fmt.Errorf("both %s and %s exist; keep only one (%s is preferred)",
				primary, legacy, ProjectConfigName)
		case hasPrimary:
			warnings, err := CheckProjectConfigTrust(e, primary)
			return primary, warnings, err
		case hasLegacy:
			warnings, err := CheckProjectConfigTrust(e, legacy)
			return legacy, warnings, err
		}
	}
	return "", nil, nil
}

// projectSearchDirs returns the directories to search, nearest first. The
// innermost directory with a workspace marker is the root, so a nested
// workspace (a MODULE.bazel below the repository root) ends the search.
// Reaching the home directory or one of its ancestors ends the walk as
// "outside a workspace".
func projectSearchDirs(e Environment, wd string, homeDirs []fs.FileInfo) ([]string, error) {
	var dirs []string
	for dir := wd; ; {
		home, err := e.isAnyOf(dir, homeDirs)
		if err != nil {
			return nil, err
		}
		if home {
			return []string{wd}, nil
		}
		dirs = append(dirs, dir)
		root, err := isWorkspaceRoot(e, dir)
		if err != nil {
			return nil, err
		}
		if root {
			return dirs, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// No workspace root above wd: only search wd itself.
			return []string{wd}, nil
		}
		dir = parent
	}
}

// isWorkspaceRoot reports whether dir holds a workspace marker file. A
// marker that is a directory is not a marker; one that cannot be checked
// is an error, so the walk never continues past an unreadable root.
// Outside Windows, a marker not owned by the current user or root is
// skipped; one whose owner cannot be determined is an error.
func isWorkspaceRoot(e Environment, dir string) (bool, error) {
	for _, m := range workspaceMarkers {
		path := filepath.Join(dir, m)
		fi, ok, err := e.stat(path)
		if err != nil {
			return false, err
		}
		if !ok || fi.IsDir() {
			continue
		}
		if e.GOOS == goosWindows {
			return true, nil
		}
		owner, known := e.FileOwner(fi)
		if !known {
			return false, fmt.Errorf("cannot determine the owner of workspace marker %s", path)
		}
		if uid := e.Getuid(); owner == uid || owner == 0 {
			return true, nil
		}
	}
	return false, nil
}
