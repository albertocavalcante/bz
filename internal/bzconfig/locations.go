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
	// Open opens a file for reading without following a final symbolic
	// link (and without blocking on a FIFO where the platform allows).
	// Config files are read only through the handle it returns.
	Open func(string) (fs.File, error)
	Stat func(string) (fs.FileInfo, error)
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
		Open:        openNoFollow,
		Stat:        os.Stat,

		EvalSymlinks: filepath.EvalSymlinks,
		SameFile:     os.SameFile,
		Getuid:       os.Getuid,
		FileOwner:    fileOwner,
	}
	env.SystemConfigPath = SystemConfigPath(env.GOOS, env.LookupEnv)
	return env
}

// WithEnv returns a copy of e whose LookupEnv reports overrides first and
// falls back to e's own lookup for other names.
func (e Environment) WithEnv(overrides map[string]string) Environment {
	inner := e.LookupEnv
	e.LookupEnv = func(name string) (string, bool) {
		if v, ok := overrides[name]; ok {
			return v, true
		}
		if inner == nil {
			return "", false
		}
		return inner(name)
	}
	return e
}

// Isolated returns a copy of e that reads no system file and looks for the
// user file under configHome (via XDG_CONFIG_HOME). Tests use it so the
// machine's real configuration cannot leak in.
func (e Environment) Isolated(configHome string) Environment {
	e = e.WithEnv(map[string]string{"XDG_CONFIG_HOME": configHome})
	e.SystemConfigPath = ""
	return e
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
		return nil, false, fmt.Errorf("checking %s: %w", DisplayPath(path), cleanErr(err))
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
		return false, fmt.Errorf("%s is not a regular file (%s)", DisplayPath(path), fi.Mode().Type())
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
//     the current user or root is skipped, with a warning, and the walk
//     continues upward, so a marker planted in a shared directory cannot
//     cut the search short or point it at a planted config. The
//     consequence: no project config applies from a workspace whose marker
//     belongs to someone else (for example a checkout reached through
//     group access, or any checkout when running as root) unless its
//     directory is listed in BZ_TRUSTED_CONFIG_DIRS, which makes markers
//     there count regardless of owner. On platforms that expose no
//     ownership at all other than Windows (such as plan9 or js), a
//     marker's owner is unknown, which is an error, so bz does not run
//     inside a workspace there.
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
		return "", nil, fmt.Errorf("finding project config: %w", cleanErr(err))
	}
	if wd, err = e.resolve(wd); err != nil {
		return "", nil, fmt.Errorf("finding project config: %w", cleanErr(err))
	}
	homeDirs, err := e.homeAndAncestors()
	if err != nil {
		return "", nil, err
	}
	trusted, warnings, err := e.trustedDirs()
	if err != nil {
		return "", warnings, err
	}
	dirs, markerWarnings, err := projectSearchDirs(e, wd, homeDirs, trusted)
	warnings = append(warnings, markerWarnings...)
	if err != nil {
		return "", warnings, err
	}
	for _, dir := range dirs {
		primary := filepath.Join(dir, ProjectConfigName)
		legacy := filepath.Join(dir, LegacyProjectConfigName)
		hasPrimary, err := e.regularFile(primary)
		if err != nil {
			return "", warnings, err
		}
		hasLegacy, err := e.regularFile(legacy)
		if err != nil {
			return "", warnings, err
		}
		switch {
		case hasPrimary && hasLegacy:
			return "", warnings, fmt.Errorf("both %s and %s exist; keep only one (%s is preferred)",
				DisplayPath(primary), DisplayPath(legacy), ProjectConfigName)
		case hasPrimary:
			return primary, warnings, e.checkTrust(primary, trusted)
		case hasLegacy:
			return legacy, warnings, e.checkTrust(legacy, trusted)
		}
	}
	return "", warnings, nil
}

// projectSearchDirs returns the directories to search, nearest first. The
// innermost directory with an accepted workspace marker is the root, so a
// nested workspace (a MODULE.bazel below the repository root) ends the
// search. Reaching the home directory or one of its ancestors ends the
// walk as "outside a workspace". It also returns one warning per skipped
// marker.
func projectSearchDirs(e Environment, wd string, homeDirs, trusted []fs.FileInfo) ([]string, []string, error) {
	var dirs, warnings []string
	for dir := wd; ; {
		home, err := e.isAnyOf(dir, homeDirs)
		if err != nil {
			return nil, warnings, err
		}
		if home {
			return []string{wd}, warnings, nil
		}
		dirs = append(dirs, dir)
		root, skipped, err := isWorkspaceRoot(e, dir, trusted)
		warnings = append(warnings, skipped...)
		if err != nil {
			return nil, warnings, err
		}
		if root {
			return dirs, warnings, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// No workspace root above wd: only search wd itself.
			return []string{wd}, warnings, nil
		}
		dir = parent
	}
}

// isWorkspaceRoot reports whether dir holds an accepted workspace marker
// file. A marker that is a directory is not a marker; one that cannot be
// checked is an error, so the walk never continues past an unreadable
// root. Outside Windows, a marker not owned by the current user or root is
// skipped with a warning, unless dir is a trusted directory; one whose
// owner cannot be determined is an error.
func isWorkspaceRoot(e Environment, dir string, trusted []fs.FileInfo) (bool, []string, error) {
	var warnings []string
	for _, m := range workspaceMarkers {
		path := filepath.Join(dir, m)
		fi, ok, err := e.stat(path)
		if err != nil {
			return false, warnings, err
		}
		if !ok || fi.IsDir() {
			continue
		}
		if e.GOOS == goosWindows {
			return true, warnings, nil
		}
		owner, known := e.FileOwner(fi)
		if !known {
			return false, warnings, fmt.Errorf("cannot determine the owner of workspace marker %s", DisplayPath(path))
		}
		if uid := e.Getuid(); owner == uid || owner == 0 {
			return true, warnings, nil
		}
		inTrusted, err := e.isAnyOf(dir, trusted)
		if err != nil {
			return false, warnings, err
		}
		if inTrusted {
			return true, warnings, nil
		}
		warnings = append(warnings, fmt.Sprintf(
			"workspace marker %s is owned by uid %d, not you or root; ignored, so no project config from that workspace applies (list %s in %s to trust it)",
			DisplayPath(path), owner, DisplayPath(dir), EnvTrustedConfigDirs))
	}
	return false, warnings, nil
}

// Loaded is the result of reading the file and environment layers.
type Loaded struct {
	// Layers holds the file and environment layers, lowest precedence first.
	Layers []Layer
	// Warnings are non-fatal problems for the caller to print once, such as
	// skipped workspace markers or unusable BZ_TRUSTED_CONFIG_DIRS entries.
	Warnings []string
	// DefaultCacheDir is the platform default cache directory.
	DefaultCacheDir string
}

// LoadLayers reads the system, user and project files and the environment.
// A missing file is skipped. Everything else that goes wrong is an error,
// so a command never runs under a configuration bz could not fully apply:
// a location error (including a stat error on a workspace marker above
// the working directory), a file that fails its trust rules, an
// unreadable or malformed file, or an invalid BZ_* boolean. Each file is
// opened once and read through the handle its checks were made on.
func LoadLayers(e Environment) (*Loaded, error) {
	home := e.home()
	out := &Loaded{DefaultCacheDir: DefaultCacheDir(e)}

	// The error comes first: FindProjectConfig can return a path together
	// with an error (an untrusted file), and that path must not be used.
	project, warnings, err := FindProjectConfig(e)
	out.Warnings = append(out.Warnings, warnings...)
	if err != nil {
		return out, err
	}
	trusted, _, err := e.trustedDirs() // warnings already reported by FindProjectConfig
	if err != nil {
		return out, err
	}
	files := []struct {
		kind LayerKind
		path string
	}{
		{LayerSystem, e.SystemConfigPath},
		{LayerUser, UserConfigPath(e)},
		{LayerProject, project},
	}
	for _, f := range files {
		if f.path == "" {
			continue
		}
		data, ok, err := e.readConfigFile(f.kind, f.path, trusted)
		if err != nil {
			return out, err
		}
		if !ok {
			continue
		}
		layer, fileWarnings, err := ParseFile(f.kind, f.path, data, home)
		out.Warnings = append(out.Warnings, fileWarnings...)
		if err != nil {
			return out, err
		}
		out.Layers = append(out.Layers, layer)
	}

	lookup := e.LookupEnv
	if lookup == nil {
		lookup = func(string) (string, bool) { return "", false }
	}
	envLayer, err := EnvLayer(lookup, home)
	if err != nil {
		return out, err
	}
	out.Layers = append(out.Layers, envLayer)
	return out, nil
}

// Resolve merges loaded layers with defaults and the flag layer. Unlike
// [Merge], it is strict: any invalid value is an error.
func (l *Loaded) Resolve(flags Layer) (*Effective, error) {
	layers := append(append([]Layer(nil), l.Layers...), flags)
	eff, err := Merge(Defaults(l.DefaultCacheDir), layers...)
	if err != nil {
		return nil, err
	}
	return eff, nil
}
