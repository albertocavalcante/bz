package bzconfig

import (
	"errors"
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
		name       string
		goos       string
		modes      map[string]fs.FileMode
		owners     map[string]int
		links      map[string]string
		files      []string
		vars       map[string]string
		noOwner    bool
		wantReason UntrustedReason // zero: trusted
		wantErr    string
	}{
		{name: "own file, private directory"},
		{name: "owned by root", owners: map[string]int{cfg: 0}},
		{name: "file and directory owned by root", owners: map[string]int{cfg: 0, "/ws": 0}},
		{
			name: "owned by another user", owners: map[string]int{cfg: 2000}, wantReason: ReasonFileOwner,
			wantErr: "owned by uid 2000, not you (uid 1000) or root; fix: remove it or have it owned by you (chown 1000 /ws/.bz.toml)",
		},
		{
			name: "group-writable file", modes: map[string]fs.FileMode{cfg: 0o664}, wantReason: ReasonFileWritable,
			wantErr: "writable by group or others (mode -rw-rw-r--); fix: chmod go-w /ws/.bz.toml,",
		},
		{name: "world-writable file", modes: map[string]fs.FileMode{cfg: 0o646}, wantReason: ReasonFileWritable},
		{
			name: "group-writable directory", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o775}, wantReason: ReasonDirWritable,
			wantErr: "its directory /ws is writable by group or others (mode -rwxrwxr-x) and is not a sticky directory holding a file you own; fix: chmod go-w /ws,",
		},
		{
			name: "own file in a directory owned by another user", owners: map[string]int{"/ws": 2000}, wantReason: ReasonDirOwner,
			wantErr: "its directory /ws is owned by uid 2000, not you (uid 1000) or root, so that user can replace the file",
		},
		{name: "sticky shared directory, own file", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | fs.ModeSticky | 0o777}, owners: map[string]int{"/ws": 0}},
		{
			name: "sticky directory owned by another user, own file", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | fs.ModeSticky | 0o777},
			owners: map[string]int{"/ws": 2000},
		},
		{
			name: "sticky shared directory, root's file", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | fs.ModeSticky | 0o777},
			owners: map[string]int{cfg: 0, "/ws": 0}, wantReason: ReasonDirWritable,
		},
		{name: "not a regular file", modes: map[string]fs.FileMode{cfg: fs.ModeNamedPipe | 0o644}, wantReason: ReasonNotRegular},
		{name: "owner unknown", noOwner: true, wantReason: ReasonOwnerUnknown},
		{
			name: "trusted directory override", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o777},
			owners: map[string]int{cfg: 2000}, vars: map[string]string{EnvTrustedConfigDirs: "/elsewhere:/ws"}, files: []string{"/elsewhere/x"},
		},
		{
			name: "override for another directory does not apply", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o777},
			vars: map[string]string{EnvTrustedConfigDirs: "/elsewhere"}, files: []string{"/elsewhere/x"}, wantReason: ReasonDirWritable,
		},
		{
			name: "override does not cover subdirectories", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o777},
			vars: map[string]string{EnvTrustedConfigDirs: "/"}, wantReason: ReasonDirWritable,
		},
		{
			name: "override through a symlinked entry", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o777},
			links: map[string]string{"/link": "/ws"}, vars: map[string]string{EnvTrustedConfigDirs: "/link"},
		},
		{
			name: "windows skips ownership and mode", goos: "windows", owners: map[string]int{cfg: 2000, "/ws": 2000},
			modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o777, cfg: 0o666},
		},
		{
			name: "symlinked config is checked at its target", links: map[string]string{cfg: "/shared/real.toml"},
			files: []string{"/shared/real.toml"}, owners: map[string]int{"/shared/real.toml": 2000}, wantReason: ReasonFileOwner,
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
			_, err := CheckProjectConfigTrust(env, cfg)
			if tt.wantReason == 0 {
				assert.NoError(t, err)
				return
			}
			var ue *UntrustedConfigError
			require.ErrorAs(t, err, &ue)
			assert.Equal(t, tt.wantReason, ue.Reason, ue.Reason.String())
			assert.ErrorIs(t, err, ErrUntrustedConfig)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Contains(t, err.Error(), EnvTrustedConfigDirs, "the error names the escape hatch")
		})
	}
}

// TestCheckProjectConfigTrust_UmaskMessage: the commonest refusal (a
// 0664 file in a 0775 directory, from umask 002) gives one actionable
// message covering both.
func TestCheckProjectConfigTrust_UmaskMessage(t *testing.T) {
	f := newFakeEnv()
	f.files = map[string]string{"/ws/.bz.toml": "", "/ws/MODULE.bazel": ""}
	f.modes["/ws/.bz.toml"] = 0o664
	f.modes["/ws"] = fs.ModeDir | 0o775

	_, err := CheckProjectConfigTrust(f.env(), "/ws/.bz.toml")
	require.EqualError(t, err, "untrusted project config /ws/.bz.toml: the file and its directory /ws are writable by group or others "+
		"(modes -rw-rw-r--, -rwxrwxr-x); fix: chmod go-w /ws/.bz.toml /ws, or list /ws in BZ_TRUSTED_CONFIG_DIRS if the shared setup is intended")
	var ue *UntrustedConfigError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, ReasonFileWritable, ue.Reason)
}

func TestCheckProjectConfigTrust_TrustedDirsEntries(t *testing.T) {
	setup := func(list string) Environment {
		f := newFakeEnv()
		f.files = map[string]string{"/ws/.bz.toml": "", "/ws/MODULE.bazel": ""}
		f.vars[EnvTrustedConfigDirs] = list
		return f.env()
	}

	for _, rel := range []string{".", "rel/dir"} {
		_, err := CheckProjectConfigTrust(setup("/ws:"+rel), "/ws/.bz.toml")
		require.Error(t, err, rel)
		assert.Contains(t, err.Error(), EnvTrustedConfigDirs)
		assert.Contains(t, err.Error(), `"`+rel+`" is not an absolute path`)
		assert.NotErrorIs(t, err, ErrUntrustedConfig)
	}

	warnings, err := CheckProjectConfigTrust(setup("/missing"), "/ws/.bz.toml")
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], `"/missing" does not exist`)

	env := setup("/broken")
	inner := env.EvalSymlinks
	env.EvalSymlinks = func(p string) (string, error) {
		if p == "/broken" {
			return "", fs.ErrPermission
		}
		return inner(p)
	}
	warnings, err = CheckProjectConfigTrust(env, "/ws/.bz.toml")
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], `"/broken" cannot be resolved`)
}

func TestFindProjectConfig_IncompleteEnvironmentFailsClosed(t *testing.T) {
	_, _, err := FindProjectConfig(Environment{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Getwd, Stat, EvalSymlinks, SameFile, Getuid, FileOwner")

	for _, drop := range []string{"SameFile", "EvalSymlinks", "Getuid", "FileOwner"} {
		env := newFakeEnv().env()
		switch drop {
		case "SameFile":
			env.SameFile = nil
		case "EvalSymlinks":
			env.EvalSymlinks = nil
		case "Getuid":
			env.Getuid = nil
		case "FileOwner":
			env.FileOwner = nil
		}
		_, _, err := FindProjectConfig(env)
		require.Error(t, err, drop)
		assert.Contains(t, err.Error(), "does not provide "+drop)
		_, err = CheckProjectConfigTrust(env, "/work/proj/.bz.toml")
		require.Error(t, err, drop)
	}

	win := newFakeEnv()
	win.goos = "windows"
	env := win.env()
	env.Getuid, env.FileOwner = nil, nil
	_, _, err = FindProjectConfig(env)
	assert.NoError(t, err, "Windows needs no ownership functions")
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
		goos    string
		want    string
		wantErr string
	}{
		{
			// Someone else's marker and config in a world-writable sticky
			// parent: the marker is skipped, so the planted config is never
			// reached (no denial of service either).
			name: "planted marker and config in a shared parent are skipped", wd: "/tmp/x",
			files:  []string{"/tmp/WORKSPACE", "/tmp/.bz.toml"},
			modes:  map[string]fs.FileMode{"/tmp": fs.ModeDir | fs.ModeSticky | 0o777},
			owners: map[string]int{"/tmp/.bz.toml": 2000, "/tmp/WORKSPACE": 2000, "/tmp": 0},
			want:   "",
		},
		{
			// A planted marker below the real root must not cut the search
			// short: the real policy still applies.
			name: "planted marker below the real root is skipped", wd: "/ws/shared/x",
			files:  []string{"/ws/MODULE.bazel", "/ws/.bz.toml", "/ws/shared/MODULE.bazel"},
			modes:  map[string]fs.FileMode{"/ws/shared": fs.ModeDir | 0o777},
			owners: map[string]int{"/ws/shared/MODULE.bazel": 2000},
			want:   "/ws/.bz.toml",
		},
		{
			name: "root-owned marker counts", wd: "/ws/pkg",
			files:  []string{"/ws/MODULE.bazel", "/ws/.bz.toml"},
			owners: map[string]int{"/ws/MODULE.bazel": 0},
			want:   "/ws/.bz.toml",
		},
		{
			name: "windows keeps markers of any owner", goos: "windows", wd: "/ws/shared/x",
			files:  []string{"/ws/MODULE.bazel", "/ws/.bz.toml", "/ws/shared/MODULE.bazel"},
			owners: map[string]int{"/ws/shared/MODULE.bazel": 2000},
			want:   "",
		},
		{
			name: "own config in sticky /tmp is trusted", wd: "/tmp/x",
			files:  []string{"/tmp/WORKSPACE", "/tmp/.bz.toml"},
			modes:  map[string]fs.FileMode{"/tmp": fs.ModeDir | fs.ModeSticky | 0o777},
			owners: map[string]int{"/tmp": 0},
			want:   "/tmp/.bz.toml",
		},
		{
			name: "another user's config in the working directory is refused", wd: "/tmp",
			files:   []string{"/tmp/.bz.toml"},
			modes:   map[string]fs.FileMode{"/tmp": fs.ModeDir | fs.ModeSticky | 0o777},
			owners:  map[string]int{"/tmp/.bz.toml": 2000, "/tmp": 0},
			wantErr: "untrusted project config /tmp/.bz.toml: owned by uid 2000",
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
			name: "home inside a workspace: that workspace's marker is ignored", home: "/ws/home/u", wd: "/ws/home/u/proj",
			files: []string{"/ws/MODULE.bazel", "/ws/.bz.toml"}, want: "",
		},
		{
			name: "unset home turns the home rule off", home: "-", wd: "/home/u/notes",
			files: []string{"/home/u/WORKSPACE", "/home/u/.bz.toml"}, want: "/home/u/.bz.toml",
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
			wantErr: "its directory /shared/repo is writable by group or others",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			f.wd = tt.wd
			switch tt.home {
			case "":
			case "-":
				f.home = ""
			default:
				f.home = tt.home
			}
			if tt.goos != "" {
				f.goos = tt.goos
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
			got, _, err := FindProjectConfig(f.env())
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

func TestFindProjectConfig_UnknownMarkerOwnerIsAnError(t *testing.T) {
	f := newFakeEnv()
	env := f.env()
	env.FileOwner = func(fs.FileInfo) (int, bool) { return 0, false }
	_, _, err := FindProjectConfig(env)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot determine the owner of workspace marker /work/proj/MODULE.bazel")
	assert.False(t, errors.Is(err, ErrUntrustedConfig))
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

	_, err := CheckProjectConfigTrust(env, path)
	require.NoError(t, err)

	require.NoError(t, os.Chmod(path, 0o666))
	_, err = CheckProjectConfigTrust(env, path)
	require.ErrorIs(t, err, ErrUntrustedConfig)
	assert.Contains(t, err.Error(), "chmod go-w")

	t.Setenv(EnvTrustedConfigDirs, dir)
	_, err = CheckProjectConfigTrust(env, path)
	assert.NoError(t, err)
}
