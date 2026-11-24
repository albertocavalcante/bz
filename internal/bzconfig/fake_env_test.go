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
	// uid is the current user; owners overrides a path's owner (default
	// uid). links maps a symlink path to its target for EvalSymlinks.
	uid    int
	owners map[string]int
	links  map[string]string
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

		uid:    1000,
		owners: map[string]int{},
		links:  map[string]string{},
	}
}

type fakeInfo struct {
	name string
	path string
	dir  bool
	mode fs.FileMode
	uid  int
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
		Stat:         f.stat,
		EvalSymlinks: f.evalSymlinks,
		SameFile: func(a, b fs.FileInfo) bool {
			fa, okA := a.(fakeInfo)
			fb, okB := b.(fakeInfo)
			return okA && okB && fa.path == fb.path
		},
		Getuid: func() int { return f.uid },
		FileOwner: func(fi fs.FileInfo) (int, bool) {
			info, ok := fi.(fakeInfo)
			return info.uid, ok
		},
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
		uid, ok := f.owners[path]
		if !ok {
			uid = f.uid
		}
		return fakeInfo{name: filepath.Base(path), path: path, dir: mode.IsDir(), mode: mode, uid: uid}, nil
	}
	if _, ok := f.files[path]; ok {
		return info(false, 0o644)
	}
	if f.dirs[path] || path == "/" || path == f.home {
		return info(true, fs.ModeDir|0o755)
	}
	for p := range f.files {
		if strings.HasPrefix(p, path+"/") {
			return info(true, fs.ModeDir|0o755)
		}
	}
	return nil, fs.ErrNotExist
}

// evalSymlinks rewrites the longest symlinked prefix of path, repeatedly.
func (f *fakeEnv) evalSymlinks(path string) (string, error) {
	for range 10 {
		changed := false
		for dir := path; ; dir = filepath.Dir(dir) {
			if target, ok := f.links[dir]; ok {
				rest := strings.TrimPrefix(path, dir)
				path = filepath.Clean(target + rest)
				changed = true
				break
			}
			if dir == filepath.Dir(dir) {
				break
			}
		}
		if !changed {
			return path, nil
		}
	}
	return "", fs.ErrInvalid
}
