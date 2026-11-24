package bzconfig

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// fakeEnv is an in-memory Environment: no test touches the real home
// directory, /etc or process environment.
type fakeEnv struct {
	goos  string
	home  string
	wd    string
	vars  map[string]string
	files map[string]string
	dirs  map[string]bool
	// modes overrides the mode of a file or directory (type and
	// permission bits); statErrs makes Stat fail for a path.
	modes    map[string]fs.FileMode
	statErrs map[string]error
}

func newFakeEnv() *fakeEnv {
	return &fakeEnv{
		goos:  "linux",
		home:  "/home/u",
		wd:    "/work/proj",
		vars:  map[string]string{},
		files: map[string]string{"/work/proj/MODULE.bazel": ""},
		dirs:  map[string]bool{},

		modes:    map[string]fs.FileMode{},
		statErrs: map[string]error{},
	}
}

type fakeInfo struct {
	name string
	dir  bool
	mode fs.FileMode
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) Mode() fs.FileMode  { return i.mode }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return i.dir }
func (i fakeInfo) Sys() any           { return nil }

func (f *fakeEnv) env() Environment {
	return Environment{
		GOOS: f.goos,
		LookupEnv: func(name string) (string, bool) {
			v, ok := f.vars[name]
			return v, ok
		},
		Getwd:       func() (string, error) { return f.wd, nil },
		UserHomeDir: func() (string, error) { return f.home, nil },
		ReadFile: func(path string) ([]byte, error) {
			if data, ok := f.files[path]; ok {
				return []byte(data), nil
			}
			return nil, fs.ErrNotExist
		},
		Stat:             f.stat,
		SystemConfigPath: "/etc/bz/config.toml",
	}
}

func (f *fakeEnv) stat(path string) (fs.FileInfo, error) {
	if err, ok := f.statErrs[path]; ok {
		return nil, err
	}
	info := func(dir bool, def fs.FileMode) (fs.FileInfo, error) {
		mode, ok := f.modes[path]
		if !ok {
			mode = def
		}
		return fakeInfo{name: filepath.Base(path), dir: mode.IsDir(), mode: mode}, nil
	}
	if _, ok := f.files[path]; ok {
		return info(false, 0o644)
	}
	if f.dirs[path] || path == "/" {
		return info(true, fs.ModeDir|0o755)
	}
	for p := range f.files {
		if strings.HasPrefix(p, path+"/") {
			return info(true, fs.ModeDir|0o755)
		}
	}
	return nil, fs.ErrNotExist
}
