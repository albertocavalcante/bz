package bzconfig

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const handleTestFile = "/work/proj/.bz.toml"

// TestReadConfigFile_ReadsTheCheckedHandle: the handle's content and owner
// differ from what a read or stat by path would see; readConfigFile must
// decide on the handle and return the handle's bytes.
func TestReadConfigFile_ReadsTheCheckedHandle(t *testing.T) {
	newEnv := func(owner int) (*fakeEnv, *[]string) {
		f := newFakeEnv()
		f.files[handleTestFile] = "by path"
		var opened []string
		f.openHook = func(path string) (fs.File, error) {
			opened = append(opened, path)
			info := fakeInfo{name: ".bz.toml", path: path, mode: 0o644, uid: owner}
			return fakeFile{Reader: strings.NewReader("from handle"), info: info}, nil
		}
		return f, &opened
	}

	f, opened := newEnv(1000)
	data, ok, err := f.env().readConfigFile(LayerProject, handleTestFile, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, f.closes, "the handle is closed")
	assert.True(t, ok)
	assert.Equal(t, "from handle", string(data))
	assert.Equal(t, []string{handleTestFile}, *opened)

	f, _ = newEnv(2000)
	_, _, err = f.env().readConfigFile(LayerProject, handleTestFile, nil)
	assert.Equal(t, 1, f.closes, "a refused handle is closed too")
	require.EqualError(t, err, "untrusted project config /work/proj/.bz.toml: owned by uid 2000, not you (uid 1000) or root; "+
		"fix: recreate it as yourself, or have an administrator change its owner (it belongs to uid 2000), "+
		"or list /work/proj in BZ_TRUSTED_CONFIG_DIRS if the shared setup is intended")
}

func TestReadConfigFile(t *testing.T) {
	tests := []struct {
		name    string
		kind    LayerKind
		path    string
		setup   func(f *fakeEnv)
		want    string
		wantOK  bool
		wantErr string
	}{
		{
			name: "missing file is absent", kind: LayerProject, path: handleTestFile,
			setup: func(f *fakeEnv) {},
		},
		{
			name: "symlink opens its target", kind: LayerProject, path: handleTestFile, wantOK: true, want: "target",
			setup: func(f *fakeEnv) {
				f.files[handleTestFile] = "link"
				f.files["/real/conf.toml"] = "target"
				f.links[handleTestFile] = "/real/conf.toml"
				f.openHook = func(path string) (fs.File, error) {
					if path != "/real/conf.toml" {
						return nil, fs.ErrPermission
					}
					return f.openPlain(path)
				}
			},
		},
		{
			// The link is resolved, then its target is replaced by a symlink
			// before the open; O_NOFOLLOW makes that open fail (ELOOP).
			name: "symlink swapped before the open is refused", kind: LayerProject, path: handleTestFile,
			setup: func(f *fakeEnv) {
				f.files[handleTestFile] = ""
				f.files["/real/conf.toml"] = ""
				f.links[handleTestFile] = "/real/conf.toml"
				f.openHook = func(path string) (fs.File, error) {
					return nil, &fs.PathError{Op: "open", Path: path, Err: errors.New("too many levels of symbolic links")}
				}
			},
			wantErr: "reading project config /work/proj/.bz.toml: open /real/conf.toml: too many levels of symbolic links",
		},
		{
			name: "file at the size limit is read", kind: LayerProject, path: handleTestFile, wantOK: true,
			want:  strings.Repeat("#", maxConfigSize),
			setup: func(f *fakeEnv) { f.files[handleTestFile] = strings.Repeat("#", maxConfigSize) },
		},
		{
			name: "project FIFO is refused", kind: LayerProject, path: handleTestFile,
			setup: func(f *fakeEnv) {
				f.files[handleTestFile] = ""
				f.modes[handleTestFile] = fs.ModeNamedPipe | 0o644
			},
			wantErr: "untrusted project config /work/proj/.bz.toml: not a regular file; fix: replace it with a regular file, " +
				"or list /work/proj in BZ_TRUSTED_CONFIG_DIRS if the shared setup is intended",
		},
		{
			name: "user config that is a directory is refused", kind: LayerUser, path: "/u/config.toml",
			setup: func(f *fakeEnv) {
				f.files["/u/config.toml/x"] = ""
			},
			wantErr: "untrusted user config /u/config.toml: not a regular file; fix: replace it with a regular file, " +
				"or list /u in BZ_TRUSTED_CONFIG_DIRS if the shared setup is intended",
		},
		{
			name: "read error after the open", kind: LayerProject, path: handleTestFile,
			setup: func(f *fakeEnv) {
				f.files[handleTestFile] = ""
				f.openHook = func(path string) (fs.File, error) {
					fi, err := f.stat(path)
					if err != nil {
						return nil, err
					}
					return failingRead{info: fi}, nil
				}
			},
			wantErr: "reading project config /work/proj/.bz.toml: read failed",
		},
		{
			name: "oversized file", kind: LayerProject, path: handleTestFile,
			setup:   func(f *fakeEnv) { f.files[handleTestFile] = strings.Repeat("#", maxConfigSize+1) },
			wantErr: "reading project config /work/proj/.bz.toml: larger than 1048576 bytes",
		},
		{
			name: "user file that is not regular", kind: LayerUser, path: "/u/config.toml",
			setup: func(f *fakeEnv) {
				f.files["/u/config.toml"] = ""
				f.modes["/u/config.toml"] = fs.ModeNamedPipe | 0o644
			},
			wantErr: "untrusted user config /u/config.toml: not a regular file; fix: replace it with a regular file, " +
				"or list /u in BZ_TRUSTED_CONFIG_DIRS if the shared setup is intended",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			tt.setup(f)
			data, ok, err := f.env().readConfigFile(tt.kind, tt.path, nil)
			assert.Equal(t, f.opens, f.closes, "every opened handle is closed exactly once")
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, string(data))
		})
	}
}

func TestReadConfigFile_IncompleteEnvironmentFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		drop func(e *Environment)
		want string
	}{
		{"no Open", func(e *Environment) { e.Open = nil }, "cannot read configuration: the environment does not provide Open"},
		{"no EvalSymlinks", func(e *Environment) { e.EvalSymlinks = nil }, "cannot read configuration: the environment does not provide EvalSymlinks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newFakeEnv().env()
			tt.drop(&env)
			_, _, err := env.readConfigFile(LayerProject, handleTestFile, nil)
			require.EqualError(t, err, tt.want)
		})
	}
}

// failingRead is a handle whose Read fails after a successful open.
type failingRead struct{ info fs.FileInfo }

func (h failingRead) Stat() (fs.FileInfo, error) { return h.info, nil }
func (failingRead) Read([]byte) (int, error)     { return 0, errors.New("read failed") }
func (failingRead) Close() error                 { return nil }

func TestTrustedDirs_MissingEntryWarns(t *testing.T) {
	f := newFakeEnv()
	f.vars[EnvTrustedConfigDirs] = "/missing"
	_, warnings, err := f.env().trustedDirs()
	require.NoError(t, err)
	assert.Equal(t, []string{`BZ_TRUSTED_CONFIG_DIRS entry "/missing" does not exist; ignored`}, warnings)
}

func TestReadConfigFile_TrustedDirectorySkipsModeRules(t *testing.T) {
	f := newFakeEnv()
	f.files[handleTestFile] = "shared"
	f.modes[handleTestFile] = 0o664
	f.modes["/work/proj"] = fs.ModeDir | 0o775
	f.vars[EnvTrustedConfigDirs] = "/work/proj"
	env := f.env()
	trusted, _, err := env.trustedDirs()
	require.NoError(t, err)

	data, ok, err := env.readConfigFile(LayerProject, handleTestFile, trusted)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "shared", string(data))
}
