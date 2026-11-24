package bzconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unicode/utf8"

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
			wantErr: "owned by uid 2000, not you (uid 1000) or root; fix: recreate it as yourself, or have an administrator change its owner (it belongs to uid 2000),",
		},
		{
			name: "group-writable file", modes: map[string]fs.FileMode{cfg: 0o664}, wantReason: ReasonFileWritable,
			wantErr: "writable by group or others (mode -rw-rw-r--); fix: chmod go-w -- '/ws/.bz.toml',",
		},
		{name: "world-writable file", modes: map[string]fs.FileMode{cfg: 0o646}, wantReason: ReasonFileWritable},
		{
			name: "group-writable directory", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o775}, wantReason: ReasonDirWritable,
			wantErr: "its directory /ws is writable by group or others (mode -rwxrwxr-x) and is not a sticky directory holding a file you own; fix: chmod go-w -- '/ws',",
		},
		{
			name: "own file in a directory owned by another user", owners: map[string]int{"/ws": 2000}, wantReason: ReasonDirOwner,
			wantErr: "its directory /ws is owned by uid 2000, not you (uid 1000) or root, so that user can replace the file",
		},
		{name: "sticky shared directory, own file", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | fs.ModeSticky | 0o777}, owners: map[string]int{"/ws": 0}},
		{
			name: "sticky directory owned by another user, own file", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | fs.ModeSticky | 0o777},
			owners: map[string]int{"/ws": 2000}, wantReason: ReasonDirOwner,
			wantErr: "its directory /ws is owned by uid 2000, not you (uid 1000) or root, so that user can replace the file; fix: keep the project in a directory you own,",
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
		"(modes -rw-rw-r--, -rwxrwxr-x); fix: chmod go-w -- '/ws/.bz.toml' '/ws', or list /ws in BZ_TRUSTED_CONFIG_DIRS if the shared setup is intended")
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

// TestCheckProjectConfigTrust_FixDependsOnOwnership: a command is
// suggested only for what the user owns; anything else is described.
func TestCheckProjectConfigTrust_FixDependsOnOwnership(t *testing.T) {
	const cfg = "/ws/.bz.toml"
	const tail = ", or list /ws in BZ_TRUSTED_CONFIG_DIRS if the shared setup is intended"
	tests := []struct {
		name   string
		modes  map[string]fs.FileMode
		owners map[string]int
		want   string
	}{
		{
			name: "file-writable, you own the file", modes: map[string]fs.FileMode{cfg: 0o664},
			want: "untrusted project config /ws/.bz.toml: writable by group or others (mode -rw-rw-r--); fix: chmod go-w -- '/ws/.bz.toml'" + tail,
		},
		{
			name: "file-writable, root owns the file", modes: map[string]fs.FileMode{cfg: 0o664}, owners: map[string]int{cfg: 0},
			want: "untrusted project config /ws/.bz.toml: writable by group or others (mode -rw-rw-r--); fix: the file is owned by root: have an administrator run chmod go-w on it, or recreate it as yourself" + tail,
		},
		{
			name: "dir-writable, you own the directory", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o775},
			want: "untrusted project config /ws/.bz.toml: its directory /ws is writable by group or others (mode -rwxrwxr-x) and is not a sticky directory holding a file you own; fix: chmod go-w -- '/ws'" + tail,
		},
		{
			name: "dir-writable, root owns the directory", modes: map[string]fs.FileMode{"/ws": fs.ModeDir | 0o775}, owners: map[string]int{"/ws": 0},
			want: "untrusted project config /ws/.bz.toml: its directory /ws is writable by group or others (mode -rwxrwxr-x) and is not a sticky directory holding a file you own; fix: its directory is owned by root: keep the project in a directory you own" + tail,
		},
		{
			name:  "root's file in root-owned sticky directory: never suggest chmod on the directory",
			modes: map[string]fs.FileMode{"/ws": fs.ModeDir | fs.ModeSticky | 0o777}, owners: map[string]int{cfg: 0, "/ws": 0},
			want: "untrusted project config /ws/.bz.toml: its directory /ws is writable by group or others (mode -rwxrwxrwx) and is not a sticky directory holding a file you own; fix: its directory is owned by root: keep the project in a directory you own" + tail,
		},
		{
			name:  "both writable, you own the file, root owns the directory",
			modes: map[string]fs.FileMode{cfg: 0o664, "/ws": fs.ModeDir | 0o775}, owners: map[string]int{"/ws": 0},
			want: "untrusted project config /ws/.bz.toml: the file and its directory /ws are writable by group or others (modes -rw-rw-r--, -rwxrwxr-x); fix: chmod go-w -- '/ws/.bz.toml'; its directory is owned by root: keep the project in a directory you own" + tail,
		},
		{
			name: "file-owner: another user's file", owners: map[string]int{cfg: 2000},
			want: "untrusted project config /ws/.bz.toml: owned by uid 2000, not you (uid 1000) or root; fix: recreate it as yourself, or have an administrator change its owner (it belongs to uid 2000)" + tail,
		},
		{
			name: "dir-owner: another user's directory", owners: map[string]int{"/ws": 2000},
			want: "untrusted project config /ws/.bz.toml: its directory /ws is owned by uid 2000, not you (uid 1000) or root, so that user can replace the file; fix: keep the project in a directory you own" + tail,
		},
		{
			name: "not-regular", modes: map[string]fs.FileMode{cfg: fs.ModeNamedPipe | 0o644},
			want: "untrusted project config /ws/.bz.toml: not a regular file; fix: replace it with a regular file" + tail,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			f.files = map[string]string{cfg: "", "/ws/MODULE.bazel": ""}
			for k, v := range tt.modes {
				f.modes[k] = v
			}
			for k, v := range tt.owners {
				f.owners[k] = v
			}
			_, err := CheckProjectConfigTrust(f.env(), cfg)
			require.EqualError(t, err, tt.want)
		})
	}
}

func TestCheckProjectConfigTrust_SuggestedPathsAreQuoted(t *testing.T) {
	const dir = "/ws dir/$(touch x)/it's/-n"
	cfg := dir + "/.bz.toml"
	f := newFakeEnv()
	f.files = map[string]string{cfg: ""}
	f.modes[cfg] = 0o664
	f.modes[dir] = fs.ModeDir | 0o775

	_, err := CheckProjectConfigTrust(f.env(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `fix: chmod go-w -- '/ws dir/$(touch x)/it'\''s/-n/.bz.toml' '/ws dir/$(touch x)/it'\''s/-n',`)
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/plain": `'/plain'`,
		"/a b":   `'/a b'`,
		"$(x)":   `'$(x)'`,
		"it's":   `'it'\''s'`,
		"-rf":    `'-rf'`,
		`"q"`:    `'"q"'`,
		"":       `''`,
	} {
		assert.Equal(t, want, shellQuote(in), in)
	}
}

func TestFindProjectConfig_SkippedMarkersWarnAndTrustedDirsCount(t *testing.T) {
	// A checkout owned by another account, reached through group access.
	setup := func() *fakeEnv {
		f := newFakeEnv()
		f.wd = "/srv/repo/pkg"
		f.files = map[string]string{"/srv/repo/MODULE.bazel": "", "/srv/repo/.bz.toml": ""}
		f.owners["/srv/repo/MODULE.bazel"] = 2000
		f.owners["/srv/repo/.bz.toml"] = 2000
		f.owners["/srv/repo"] = 2000
		return f
	}

	got, warnings, err := FindProjectConfig(setup().env())
	require.NoError(t, err)
	assert.Empty(t, got, "the foreign workspace's policy does not apply")
	require.Len(t, warnings, 1)
	assert.Equal(t, "workspace marker /srv/repo/MODULE.bazel is owned by uid 2000, not you or root; ignored, "+
		"so no project config from that workspace applies (list /srv/repo in BZ_TRUSTED_CONFIG_DIRS to trust it)", warnings[0])

	f := setup()
	f.vars[EnvTrustedConfigDirs] = "/srv/repo"
	got, warnings, err = FindProjectConfig(f.env())
	require.NoError(t, err)
	assert.Equal(t, "/srv/repo/.bz.toml", got, "a trusted directory's marker counts and its config is trusted")
	assert.Empty(t, warnings)
}

// assertNoControlBytes fails if s holds any byte below 0x20 (newline
// included), DEL, or invalid UTF-8.
func assertNoControlBytes(t *testing.T, s string) {
	t.Helper()
	assert.True(t, utf8.ValidString(s), "invalid UTF-8 in %q", s)
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			t.Fatalf("control byte %#x at %d in %q", s[i], i, s)
		}
	}
}

func TestMessagesNeverEmitControlBytes(t *testing.T) {
	const dir = "/w'q\nz\x1b[31m\x07\xff"
	cfg := dir + "/.bz.toml"

	t.Run("untrusted config error and its fix", func(t *testing.T) {
		f := newFakeEnv()
		f.files = map[string]string{cfg: ""}
		f.modes[cfg] = 0o664
		f.modes[dir] = fs.ModeDir | 0o775
		_, err := CheckProjectConfigTrust(f.env(), cfg)
		require.Error(t, err)
		msg := err.Error()
		assertNoControlBytes(t, msg)
		assert.Contains(t, msg, `"/w'q\nz\x1b[31m\a\xff/.bz.toml"`, "Go-quoted path")
		assert.NotContains(t, msg, "chmod", "no copyable command for such a path")
		assert.Contains(t, msg, "rename the file or directory to remove control or non-printable characters")
	})

	t.Run("other refusal reasons", func(t *testing.T) {
		for _, owners := range []map[string]int{{cfg: 2000}, {dir: 2000}} {
			f := newFakeEnv()
			f.files = map[string]string{cfg: ""}
			for k, v := range owners {
				f.owners[k] = v
			}
			_, err := CheckProjectConfigTrust(f.env(), cfg)
			require.Error(t, err)
			assertNoControlBytes(t, err.Error())
		}
	})

	t.Run("symlink target", func(t *testing.T) {
		f := newFakeEnv()
		f.files = map[string]string{"/ws/.bz.toml": "", cfg: ""}
		f.links["/ws/.bz.toml"] = cfg
		f.owners[cfg] = 2000
		_, err := CheckProjectConfigTrust(f.env(), "/ws/.bz.toml")
		require.Error(t, err)
		assertNoControlBytes(t, err.Error())
	})

	t.Run("skipped marker warning", func(t *testing.T) {
		f := newFakeEnv()
		f.wd = dir + "/pkg"
		f.files = map[string]string{dir + "/MODULE.bazel": ""}
		f.owners[dir+"/MODULE.bazel"] = 2000
		_, warnings, err := FindProjectConfig(f.env())
		require.NoError(t, err)
		require.Len(t, warnings, 1)
		assertNoControlBytes(t, warnings[0])
	})

	t.Run("stat, both-names and marker-owner errors", func(t *testing.T) {
		f := newFakeEnv()
		f.wd = dir
		f.files = map[string]string{dir + "/MODULE.bazel": ""}
		f.statErrs[cfg] = &fs.PathError{Op: "stat", Path: cfg, Err: fs.ErrPermission}
		_, _, err := FindProjectConfig(f.env())
		require.Error(t, err)
		assertNoControlBytes(t, err.Error())
		assert.ErrorIs(t, err, fs.ErrPermission)

		f = newFakeEnv()
		f.wd = dir
		f.files = map[string]string{dir + "/MODULE.bazel": "", cfg: "", dir + "/.bzconfig.toml": ""}
		_, _, err = FindProjectConfig(f.env())
		require.Error(t, err)
		assertNoControlBytes(t, err.Error())

		env := newFakeEnv()
		env.wd = dir
		env.files = map[string]string{dir + "/MODULE.bazel": ""}
		e := env.env()
		e.FileOwner = func(fs.FileInfo) (int, bool) { return 0, false }
		_, _, err = FindProjectConfig(e)
		require.Error(t, err)
		assertNoControlBytes(t, err.Error())
	})

	t.Run("trusted-dirs entries", func(t *testing.T) {
		f := newFakeEnv()
		f.files = map[string]string{"/ws/.bz.toml": ""}
		f.vars[EnvTrustedConfigDirs] = "/missing\x1b[2J"
		warnings, err := CheckProjectConfigTrust(f.env(), "/ws/.bz.toml")
		require.NoError(t, err)
		require.Len(t, warnings, 1)
		assertNoControlBytes(t, warnings[0])

		f.vars[EnvTrustedConfigDirs] = "rel\nfake line"
		_, err = CheckProjectConfigTrust(f.env(), "/ws/.bz.toml")
		require.Error(t, err)
		assertNoControlBytes(t, err.Error())
	})
}

func TestDisplayPath(t *testing.T) {
	for in, want := range map[string]string{
		"/plain/path":     "/plain/path",
		"/with space/ünï": "/with space/ünï",
		"/a\nb":           `"/a\nb"`,
		"/esc\x1b[31m":    `"/esc\x1b[31m"`,
		"/bad\xff":        `"/bad\xff"`,
		"/del\x7f":        `"/del\x7f"`,
		"/nbsp\u00a0":     `"/nbsp\u00a0"`,
	} {
		assert.Equal(t, want, displayPath(in), "%q", in)
	}
}
