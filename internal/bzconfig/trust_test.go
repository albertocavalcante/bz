package bzconfig

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckProjectConfigTrust(t *testing.T) {
	const cfg = "/ws/.bz.toml"
	tests := []struct {
		name    string
		goos    string
		modes   map[string]fs.FileMode
		owners  map[string]int
		links   map[string]string
		files   []string
		vars    map[string]string
		noOwner bool
		wantErr string // empty: trusted
	}{
		{name: "own file, private directory"},
		{name: "owned by root", owners: map[string]int{cfg: 0}},
		{name: "owned by another user", owners: map[string]int{cfg: 2000}, wantErr: "owned by uid 2000, not the current user (uid 1000) or root"},
		{name: "group-writable file", modes: map[string]fs.FileMode{cfg: 0o664}, wantErr: "writable by group or others (mode -rw-rw-r--)"},
		{name: "world-writable file", modes: map[string]fs.FileMode{cfg: 0o646}, wantErr: "writable by group or others"},
		{
			name: "group-writable directory", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o775},
			wantErr: "directory /ws is writable by group or others",
		},
		{name: "sticky shared directory, own file", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | fs.ModeSticky | 0o777}},
		{
			name: "sticky shared directory, root's file", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | fs.ModeSticky | 0o777},
			owners: map[string]int{cfg: 0}, wantErr: "is not a sticky directory holding a file you own",
		},
		{name: "not a regular file", modes: map[string]fs.FileMode{cfg: fs.ModeNamedPipe | 0o644}, wantErr: "not a regular file"},
		{name: "owner unknown", noOwner: true, wantErr: "its owner cannot be determined"},
		{
			name: "trusted directory override", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o777},
			owners: map[string]int{cfg: 2000}, vars: map[string]string{EnvTrustedConfigDirs: "/elsewhere:/ws"},
		},
		{
			name: "override for another directory does not apply", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o777},
			vars: map[string]string{EnvTrustedConfigDirs: "/elsewhere"}, wantErr: "directory /ws is writable",
		},
		{
			name: "override through a symlinked entry", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o777},
			links: map[string]string{"/link": "/ws"}, vars: map[string]string{EnvTrustedConfigDirs: "/link"},
		},
		{
			name: "windows skips ownership and mode", goos: "windows", owners: map[string]int{cfg: 2000},
			modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o777, cfg: 0o666},
		},
		{
			name: "symlinked config is checked at its target", links: map[string]string{cfg: "/shared/real.toml"},
			files: []string{"/shared/real.toml"}, owners: map[string]int{"/shared/real.toml": 2000},
			wantErr: "/ws/.bz.toml (resolved to /shared/real.toml): owned by uid 2000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			f.files = map[string]string{cfg: "", "/ws/MODULE.bazel": ""}
			if tt.goos != "" {
				f.goos = tt.goos
			}
			for _, p := range tt.files {
				f.files[p] = ""
			}
			for k, v := range tt.modes {
				f.modes[k] = v
			}
			for k, v := range tt.owners {
				f.owners[k] = v
			}
			for k, v := range tt.links {
				f.links[k] = v
			}
			for k, v := range tt.vars {
				f.vars[k] = v
			}
			env := f.env()
			if tt.noOwner {
				env.FileOwner = func(fs.FileInfo) (int, bool) { return 0, false }
			}
			err := CheckProjectConfigTrust(env, cfg)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Contains(t, err.Error(), "untrusted project config")
			assert.Contains(t, err.Error(), EnvTrustedConfigDirs, "the error names the escape hatch")
		})
	}
}

func TestFindProjectConfig_TrustRules(t *testing.T) {
	tests := []struct {
		name    string
		home    string
		wd      string
		files   []string
		modes   map[string]fs.FileMode
		owners  map[string]int
		links   map[string]string
		vars    map[string]string
		want    string
		wantErr string
	}{
		{
			// Anyone can create /tmp/WORKSPACE and /tmp/.bz.toml; a user
			// running bz in /tmp/x must not run under that policy.
			name: "planted /tmp workspace and config is refused", wd: "/tmp/x",
			files:   []string{"/tmp/WORKSPACE", "/tmp/.bz.toml"},
			modes:   map[string]fs.FileMode{"/tmp": fs.ModeDir | fs.ModeSticky | 0o777},
			owners:  map[string]int{"/tmp/.bz.toml": 2000, "/tmp/WORKSPACE": 2000},
			wantErr: "untrusted project config /tmp/.bz.toml: owned by uid 2000",
		},
		{
			name: "own config in sticky /tmp is trusted", wd: "/tmp/x",
			files: []string{"/tmp/WORKSPACE", "/tmp/.bz.toml"},
			modes: map[string]fs.FileMode{"/tmp": fs.ModeDir | fs.ModeSticky | 0o777},
			want:  "/tmp/.bz.toml",
		},
		{
			name: "~/WORKSPACE is not a workspace root", wd: "/home/u/notes/sub",
			files: []string{"/home/u/WORKSPACE", "/home/u/.bz.toml"}, want: "",
		},
		{
			name: "marker in an ancestor of home is ignored", wd: "/home/u/notes",
			files: []string{"/home/MODULE.bazel", "/home/.bz.toml"}, want: "",
		},
		{
			name: "working directory is home: only home is searched", wd: "/home/u",
			files: []string{"/home/u/MODULE.bazel", "/home/u/.bz.toml"}, want: "/home/u/.bz.toml",
		},
		{
			name: "workspace below home still works", wd: "/home/u/src/repo/pkg",
			files: []string{"/home/u/src/repo/MODULE.bazel", "/home/u/src/repo/.bz.toml", "/home/u/.bz.toml"},
			want:  "/home/u/src/repo/.bz.toml",
		},
		{
			name: "symlinked home is recognised by identity", home: "/home/u", wd: "/data/u/notes",
			links: map[string]string{"/home/u": "/data/u"},
			files: []string{"/data/u/MODULE.bazel", "/data/u/.bz.toml"}, want: "",
		},
		{
			name: "symlinked working directory: the physical path is searched", wd: "/link/pkg",
			links: map[string]string{"/link": "/real/ws"},
			files: []string{"/real/ws/MODULE.bazel", "/real/ws/.bz.toml"}, want: "/real/ws/.bz.toml",
		},
		{
			name: "nested workspace: the inner root wins", wd: "/repo/examples/app",
			files: []string{"/repo/MODULE.bazel", "/repo/.bz.toml", "/repo/examples/MODULE.bazel", "/repo/examples/.bz.toml"},
			want:  "/repo/examples/.bz.toml",
		},
		{
			name: "shared group-writable checkout via BZ_TRUSTED_CONFIG_DIRS", wd: "/shared/repo",
			files: []string{"/shared/repo/MODULE.bazel", "/shared/repo/.bz.toml"},
			modes: map[string]fs.FileMode{"/shared/repo": fs.ModeDir | 0o775, "/shared/repo/.bz.toml": 0o664},
			vars:  map[string]string{EnvTrustedConfigDirs: "/shared/repo"}, want: "/shared/repo/.bz.toml",
		},
		{
			name: "shared group-writable checkout without the override", wd: "/shared/repo",
			files:   []string{"/shared/repo/MODULE.bazel", "/shared/repo/.bz.toml"},
			modes:   map[string]fs.FileMode{"/shared/repo": fs.ModeDir | 0o775},
			wantErr: "directory /shared/repo is writable by group or others",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			f.wd = tt.wd
			if tt.home != "" {
				f.home = tt.home
			}
			f.files = map[string]string{}
			for _, p := range tt.files {
				f.files[p] = ""
			}
			for k, v := range tt.modes {
				f.modes[k] = v
			}
			for k, v := range tt.owners {
				f.owners[k] = v
			}
			for k, v := range tt.links {
				f.links[k] = v
			}
			for k, v := range tt.vars {
				f.vars[k] = v
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

// TestCheckProjectConfigTrust_RealFilesystem exercises the OS environment
// (owner lookup and mode bits) on a temporary directory.
func TestCheckProjectConfigTrust_RealFilesystem(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ownership and mode checks do not apply on Windows")
	}
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	path := filepath.Join(dir, ProjectConfigName)
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	env := OSEnvironment()

	require.NoError(t, CheckProjectConfigTrust(env, path))

	require.NoError(t, os.Chmod(path, 0o666))
	err := CheckProjectConfigTrust(env, path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writable by group or others")

	t.Setenv(EnvTrustedConfigDirs, dir)
	assert.NoError(t, CheckProjectConfigTrust(env, path))
}
