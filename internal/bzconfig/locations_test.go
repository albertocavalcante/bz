package bzconfig

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemConfigPath(t *testing.T) {
	lookup := func(vars map[string]string) func(string) (string, bool) {
		return func(n string) (string, bool) { v, ok := vars[n]; return v, ok }
	}
	assert.Equal(t, "/etc/bz/config.toml", SystemConfigPath("linux", lookup(nil)))
	assert.Equal(t, "/etc/bz/config.toml", SystemConfigPath("darwin", lookup(nil)))
	assert.Equal(t, filepath.Join(`C:\ProgramData`, "bz", "config.toml"),
		SystemConfigPath("windows", lookup(map[string]string{"ProgramData": `C:\ProgramData`})))
	assert.Empty(t, SystemConfigPath("windows", lookup(nil)))
	assert.Empty(t, SystemConfigPath("windows", nil), "nil lookup must not panic")
	assert.Equal(t, "/etc/bz/config.toml", SystemConfigPath("linux", nil))
}

func TestUserConfigPath(t *testing.T) {
	tests := []struct {
		name string
		goos string
		home string // "-" means no home directory
		vars map[string]string
		want string
	}{
		{name: "linux default", goos: "linux", want: "/home/u/.config/bz/config.toml"},
		{name: "macOS keeps ~/.config", goos: "darwin", want: "/home/u/.config/bz/config.toml"},
		{name: "XDG_CONFIG_HOME", goos: "darwin", vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}, want: "/xdg/bz/config.toml"},
		{name: "relative XDG ignored", goos: "linux", vars: map[string]string{"XDG_CONFIG_HOME": "rel"}, want: "/home/u/.config/bz/config.toml"},
		{name: "windows AppData", goos: "windows", vars: map[string]string{"AppData": "/appdata"}, want: filepath.Join("/appdata", "bz", "config.toml")},
		{name: "windows without AppData", goos: "windows", want: "/home/u/.config/bz/config.toml"},
		{name: "empty home", goos: "linux", home: "-", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			f.goos = tt.goos
			if tt.home == "-" {
				f.home = ""
			}
			for k, v := range tt.vars {
				f.vars[k] = v
			}
			assert.Equal(t, tt.want, UserConfigPath(f.env()))
		})
	}
}

func TestDefaultCacheDir(t *testing.T) {
	tests := []struct {
		name         string
		goos         string
		vars         map[string]string
		legacyExists bool
		legacyMode   fs.FileMode // when set, ~/.cache/bz exists with this mode
		legacyErr    error       // when set, checking ~/.cache/bz fails
		noHome       bool
		want         string
	}{
		{name: "linux default", goos: "linux", want: "/home/u/.cache/bz"},
		{name: "macOS default", goos: "darwin", want: "/home/u/.cache/bz"},
		{name: "XDG_CACHE_HOME", goos: "linux", vars: map[string]string{"XDG_CACHE_HOME": "/xdg-cache"}, want: "/xdg-cache/bz"},
		{
			name: "existing ~/.cache/bz is kept over XDG", goos: "linux", legacyExists: true,
			vars: map[string]string{"XDG_CACHE_HOME": "/xdg-cache"}, want: "/home/u/.cache/bz",
		},
		{name: "windows LocalAppData", goos: "windows", vars: map[string]string{"LocalAppData": "/local"}, want: filepath.Join("/local", "bz", "cache")},
		{
			name: "windows keeps existing ~/.cache/bz", goos: "windows", legacyExists: true,
			vars: map[string]string{"LocalAppData": "/local"}, want: "/home/u/.cache/bz",
		},
		{name: "relative XDG_CACHE_HOME ignored", goos: "linux", vars: map[string]string{"XDG_CACHE_HOME": "rel"}, want: "/home/u/.cache/bz"},
		{
			name: "~/.cache/bz that is a file is not the legacy cache", goos: "linux", legacyMode: 0o644,
			vars: map[string]string{"XDG_CACHE_HOME": "/xdg-cache"}, want: "/xdg-cache/bz",
		},
		{
			name: "~/.cache/bz that cannot be checked is kept", goos: "linux", legacyErr: fs.ErrPermission,
			vars: map[string]string{"XDG_CACHE_HOME": "/xdg-cache"}, want: "/home/u/.cache/bz",
		},
		{name: "empty home", goos: "linux", noHome: true, want: filepath.Join(".cache", "bz")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			f.goos = tt.goos
			for k, v := range tt.vars {
				f.vars[k] = v
			}
			if tt.legacyExists {
				f.dirs["/home/u/.cache/bz"] = true
			}
			if tt.legacyMode != 0 {
				f.files["/home/u/.cache/bz"] = ""
				f.modes["/home/u/.cache/bz"] = tt.legacyMode
			}
			if tt.legacyErr != nil {
				f.statErrs["/home/u/.cache/bz"] = tt.legacyErr
			}
			if tt.noHome {
				f.home = ""
			}
			assert.Equal(t, tt.want, DefaultCacheDir(f.env()))
		})
	}
}

func TestFindProjectConfig(t *testing.T) {
	tests := []struct {
		name     string
		wd       string
		files    []string
		modes    map[string]fs.FileMode
		statErrs map[string]error
		want     string
		wantErr  string
	}{
		{name: "none", wd: "/ws/a", files: []string{"/ws/MODULE.bazel"}, want: ""},
		{name: "documented name", wd: "/ws", files: []string{"/ws/MODULE.bazel", "/ws/.bz.toml"}, want: "/ws/.bz.toml"},
		{name: "legacy name", wd: "/ws", files: []string{"/ws/MODULE.bazel", "/ws/.bzconfig.toml"}, want: "/ws/.bzconfig.toml"},
		{
			name: "both names is an error", wd: "/ws",
			files:   []string{"/ws/MODULE.bazel", "/ws/.bz.toml", "/ws/.bzconfig.toml"},
			wantErr: "both /ws/.bz.toml and /ws/.bzconfig.toml exist; keep only one (.bz.toml is preferred)",
		},
		{
			name: "marker that is a directory is not a root", wd: "/outer/ws/pkg",
			files: []string{"/outer/ws/MODULE.bazel/x", "/outer/MODULE.bazel", "/outer/.bz.toml"}, want: "/outer/.bz.toml",
		},
		{
			name: "config path that is a directory is an error", wd: "/ws",
			files: []string{"/ws/MODULE.bazel", "/ws/.bz.toml/x"}, wantErr: "/ws/.bz.toml is not a regular file",
		},
		{
			name: "config path that is a FIFO is an error", wd: "/ws",
			files: []string{"/ws/MODULE.bazel", "/ws/.bzconfig.toml"}, modes: map[string]fs.FileMode{"/ws/.bzconfig.toml": fs.ModeNamedPipe | 0o644},
			wantErr: "/ws/.bzconfig.toml is not a regular file",
		},
		{
			name: "config that cannot be checked is an error", wd: "/ws",
			files: []string{"/ws/MODULE.bazel"}, statErrs: map[string]error{"/ws/.bz.toml": fs.ErrPermission},
			wantErr: "checking /ws/.bz.toml: permission denied",
		},
		{
			name: "marker that cannot be checked is an error", wd: "/ws/pkg",
			files: []string{"/ws/MODULE.bazel", "/ws/.bz.toml"}, statErrs: map[string]error{"/ws/pkg/WORKSPACE": fs.ErrPermission},
			wantErr: "checking /ws/pkg/WORKSPACE: permission denied",
		},
		{name: "working directory is /", wd: "/", files: []string{"/.bz.toml"}, want: "/.bz.toml"},
		{
			name: "nested workspace: the inner root wins", wd: "/repo/examples/app",
			files: []string{"/repo/MODULE.bazel", "/repo/.bz.toml", "/repo/examples/MODULE.bazel"}, want: "",
		},
		{
			name: "found at workspace root from a subdirectory", wd: "/ws/pkg/sub",
			files: []string{"/ws/MODULE.bazel", "/ws/.bz.toml"}, want: "/ws/.bz.toml",
		},
		{
			name: "nearest wins", wd: "/ws/pkg",
			files: []string{"/ws/MODULE.bazel", "/ws/.bz.toml", "/ws/pkg/.bzconfig.toml"}, want: "/ws/pkg/.bzconfig.toml",
		},
		{
			name: "search stops at the workspace root", wd: "/outer/ws/pkg",
			files: []string{"/outer/.bz.toml", "/outer/ws/WORKSPACE"}, want: "",
		},
		{
			name: "outside a workspace only the working directory counts", wd: "/plain/sub",
			files: []string{"/plain/.bz.toml"}, want: "",
		},
		{
			name: "outside a workspace, file in working directory", wd: "/plain",
			files: []string{"/plain/.bz.toml"}, want: "/plain/.bz.toml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			f.wd = tt.wd
			f.files = map[string]string{}
			for _, p := range tt.files {
				f.files[p] = ""
			}
			for p, m := range tt.modes {
				f.modes[p] = m
			}
			for p, e := range tt.statErrs {
				f.statErrs[p] = e
			}
			got, _, err := FindProjectConfig(f.env())
			if tt.wantErr != "" {
				require.Error(t, err)
				if strings.HasPrefix(tt.wantErr, "both ") {
					assert.EqualError(t, err, tt.wantErr)
				} else {
					assert.Contains(t, err.Error(), tt.wantErr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
