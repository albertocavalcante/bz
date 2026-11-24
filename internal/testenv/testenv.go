// Package testenv gives test binaries a process environment free of the
// developer's own configuration.
package testenv

import (
	"os"
	"path/filepath"
	"strings"
)

// Isolate unsets every BZ_* variable and points the home and config/cache
// directory variables at a fresh temporary directory. Call it from
// TestMain; restore undoes both and removes the directory.
func Isolate() (dir string, restore func(), err error) {
	dir, err = os.MkdirTemp("", "bz-test-home-")
	if err != nil {
		return "", nil, err
	}
	saved := map[string]*string{}
	set := func(name string, value *string) {
		if _, done := saved[name]; !done {
			if old, ok := os.LookupEnv(name); ok {
				saved[name] = &old
			} else {
				saved[name] = nil
			}
		}
		if value == nil {
			_ = os.Unsetenv(name)
		} else {
			_ = os.Setenv(name, *value)
		}
	}
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "BZ_") {
			set(name, nil)
		}
	}
	for name, sub := range map[string]string{
		"HOME": "", "USERPROFILE": "", // os.UserHomeDir on Unix and Windows
		"XDG_CONFIG_HOME": "config", "XDG_CACHE_HOME": "cache",
		"AppData": "appdata", "LocalAppData": "localappdata",
	} {
		v := filepath.Join(dir, sub)
		set(name, &v)
	}
	return dir, func() {
		for name, old := range saved {
			if old == nil {
				_ = os.Unsetenv(name)
			} else {
				_ = os.Setenv(name, *old)
			}
		}
		_ = os.RemoveAll(dir)
	}, nil
}
