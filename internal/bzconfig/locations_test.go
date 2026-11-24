package bzconfig

import (
	"path/filepath"
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
}

func TestUserConfigPath(t *testing.T) {
	tests := []struct {
		name string
		goos string
		vars map[string]string
		want string
	}{
		{name: "linux default", goos: "linux", want: "/home/u/.config/bz/config.toml"},
		{name: "macOS keeps ~/.config", goos: "darwin", want: "/home/u/.config/bz/config.toml"},
		{name: "XDG_CONFIG_HOME", goos: "darwin", vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}, want: "/xdg/bz/config.toml"},
		{name: "relative XDG ignored", goos: "linux", vars: map[string]string{"XDG_CONFIG_HOME": "rel"}, want: "/home/u/.config/bz/config.toml"},
		{name: "windows AppData", goos: "windows", vars: map[string]string{"AppData": "/appdata"}, want: filepath.Join("/appdata", "bz", "config.toml")},
		{name: "windows without AppData", goos: "windows", want: "/home/u/.config/bz/config.toml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			f.goos = tt.goos
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
			assert.Equal(t, tt.want, DefaultCacheDir(f.env()))
		})
	}
}

func TestFindProjectConfig(t *testing.T) {
	tests := []struct {
		name    string
		wd      string
		files   []string
		want    string
		wantErr string
	}{
		{name: "none", wd: "/ws/a", files: []string{"/ws/MODULE.bazel"}, want: ""},
		{name: "documented name", wd: "/ws", files: []string{"/ws/MODULE.bazel", "/ws/.bz.toml"}, want: "/ws/.bz.toml"},
		{name: "legacy name", wd: "/ws", files: []string{"/ws/MODULE.bazel", "/ws/.bzconfig.toml"}, want: "/ws/.bzconfig.toml"},
		{
			name: "both names is an error", wd: "/ws",
			files:   []string{"/ws/MODULE.bazel", "/ws/.bz.toml", "/ws/.bzconfig.toml"},
			wantErr: "keep only one",
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
			got, err := FindProjectConfig(f.env())
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
