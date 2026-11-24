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
}

func newFakeEnv() *fakeEnv {
	return &fakeEnv{
		goos:  "linux",
		home:  "/home/u",
		wd:    "/work/proj",
		vars:  map[string]string{},
		files: map[string]string{"/work/proj/MODULE.bazel": ""},
		dirs:  map[string]bool{},
	}
}

type fakeInfo struct {
	name string
	dir  bool
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) Mode() fs.FileMode  { return 0 }
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
		Stat: func(path string) (fs.FileInfo, error) {
			if _, ok := f.files[path]; ok {
				return fakeInfo{name: filepath.Base(path)}, nil
			}
			if f.dirs[path] {
				return fakeInfo{name: filepath.Base(path), dir: true}, nil
			}
			for p := range f.files {
				if strings.HasPrefix(p, path+"/") {
					return fakeInfo{name: filepath.Base(path), dir: true}, nil
				}
			}
			return nil, fs.ErrNotExist
		},
		SystemConfigPath: "/etc/bz/config.toml",
	}
}
