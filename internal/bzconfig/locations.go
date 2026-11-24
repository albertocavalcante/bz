package bzconfig

import (
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

func (e Environment) isDir(path string) bool {
	if e.Stat == nil {
		return false
	}
	fi, err := e.Stat(path)
	return err == nil && fi.IsDir()
}

func (e Environment) isFile(path string) bool {
	if e.Stat == nil {
		return false
	}
	fi, err := e.Stat(path)
	return err == nil && !fi.IsDir()
}

// SystemConfigPath returns the system-wide config file:
// /etc/bz/config.toml, or %ProgramData%\bz\config.toml on Windows (none
// when ProgramData is unset).
func SystemConfigPath(goos string, lookup func(string) (string, bool)) string {
	if goos == "windows" {
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
	if e.GOOS == "windows" {
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
//     earlier release used, so existing caches are never orphaned);
//  2. $XDG_CACHE_HOME/bz when XDG_CACHE_HOME is an absolute path;
//  3. on Windows, %LocalAppData%\bz\cache;
//  4. otherwise ~/.cache/bz.
func DefaultCacheDir(e Environment) string {
	home := e.home()
	legacy := ""
	if home != "" {
		legacy = filepath.Join(home, ".cache", "bz")
		if e.isDir(legacy) {
			return legacy
		}
	}
	if xdg := e.getenv("XDG_CACHE_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "bz")
	}
	if e.GOOS == "windows" {
		if local := e.getenv("LocalAppData"); local != "" {
			return filepath.Join(local, "bz", "cache")
		}
	}
	if legacy != "" {
		return legacy
	}
	return filepath.Join(".cache", "bz")
}

// FindProjectConfig searches for the project config file.
//
// When the working directory is inside a Bazel workspace (a directory
// containing MODULE.bazel, REPO.bazel, WORKSPACE.bazel or WORKSPACE), every
// directory from the working directory up to and including the workspace
// root is searched and the nearest file wins. Outside a workspace only the
// working directory is searched, so a stray file in a parent directory is
// never picked up. It returns "" when there is no project file.
func FindProjectConfig(e Environment) (string, error) {
	if e.Getwd == nil {
		return "", nil
	}
	wd, err := e.Getwd()
	if err != nil {
		return "", fmt.Errorf("finding project config: %w", err)
	}
	for _, dir := range projectSearchDirs(e, wd) {
		primary := filepath.Join(dir, ProjectConfigName)
		legacy := filepath.Join(dir, LegacyProjectConfigName)
		hasPrimary, hasLegacy := e.isFile(primary), e.isFile(legacy)
		switch {
		case hasPrimary && hasLegacy:
			return "", fmt.Errorf("both %s and %s exist; keep only one (%s is preferred)",
				primary, legacy, ProjectConfigName)
		case hasPrimary:
			return primary, nil
		case hasLegacy:
			return legacy, nil
		}
	}
	return "", nil
}

func projectSearchDirs(e Environment, wd string) []string {
	var dirs []string
	for dir := wd; ; {
		dirs = append(dirs, dir)
		if isWorkspaceRoot(e, dir) {
			return dirs
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// No workspace root above wd: only search wd itself.
			return []string{wd}
		}
		dir = parent
	}
}

func isWorkspaceRoot(e Environment, dir string) bool {
	for _, m := range workspaceMarkers {
		if e.isFile(filepath.Join(dir, m)) {
			return true
		}
	}
	return false
}
