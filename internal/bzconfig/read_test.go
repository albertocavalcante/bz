package bzconfig

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_ReadsTheCheckedHandle uses the Open seam: the handle's content
// and FileInfo differ from what a stat or read by path would see. The
// loader must decide on the handle's FileInfo and read the handle's bytes.
func TestLoad_ReadsTheCheckedHandle(t *testing.T) {
	newEnv := func(handleData string, handleOwner int) (*fakeEnv, *[]string) {
		f := newFakeEnv()
		f.files[projectFile] = "[network]\nmode = \"offline\"\n" // what a re-open by path would read
		var opened []string
		f.openHook = func(path string) (fs.File, error) {
			opened = append(opened, path)
			if path != projectFile {
				return f.openPlain(path)
			}
			info := fakeInfo{name: ".bz.toml", path: path, mode: 0o644, uid: handleOwner}
			return fakeFile{Reader: strings.NewReader(handleData), info: info}, nil
		}
		return f, &opened
	}

	f, opened := newEnv("[commands]\ndisabled = [\"audit\"]\n", 1000)
	eff := mustLoad(t, f, Layer{Kind: LayerFlag})
	assert.Equal(t, []string{"audit"}, eff.Commands.Disabled, "content comes from the handle")
	assert.Equal(t, NetworkModeOnline, eff.Network.Mode, "the by-path content is never read")
	assert.Equal(t, 1, strings.Count(strings.Join(*opened, "\n"), projectFile), "opened exactly once")

	// The path looks trusted, but the opened handle belongs to someone else.
	f, _ = newEnv("[commands]\ndisabled = []\n", 2000)
	_, _, err := Load(f.env(), Layer{Kind: LayerFlag})
	require.ErrorIs(t, err, ErrUntrustedConfig)
	assert.Contains(t, err.Error(), "owned by uid 2000")
}

func TestLoad_SymlinkedConfigOpensTheResolvedTarget(t *testing.T) {
	f := newFakeEnv()
	f.files[projectFile] = ""
	f.files["/real/conf.toml"] = "[commands]\ndisabled = [\"sbom\"]\n"
	f.links[projectFile] = "/real/conf.toml"
	var opened []string
	f.openHook = func(path string) (fs.File, error) {
		opened = append(opened, path)
		return f.openPlain(path)
	}
	eff := mustLoad(t, f, Layer{Kind: LayerFlag})
	assert.Equal(t, []string{"sbom"}, eff.Commands.Disabled)
	assert.Contains(t, opened, "/real/conf.toml")
	assert.NotContains(t, opened, projectFile, "the link itself is never opened")
}

func TestLoad_OversizedFileIsAnError(t *testing.T) {
	f := newFakeEnv()
	f.files[projectFile] = strings.Repeat("#", maxConfigSize+1)
	_, _, err := Load(f.env(), Layer{Kind: LayerFlag})
	require.EqualError(t, err, "reading project config /work/proj/.bz.toml: larger than 1048576 bytes")
}

func TestLoad_NonRegularUserFileIsAnError(t *testing.T) {
	f := newFakeEnv()
	f.files[userFile] = ""
	f.modes[userFile] = fs.ModeNamedPipe | 0o644
	_, _, err := Load(f.env(), Layer{Kind: LayerFlag})
	require.EqualError(t, err, "reading user config /home/u/.config/bz/config.toml: not a regular file")
}

func TestLoad_WarningsAreReturned(t *testing.T) {
	f := newFakeEnv()
	f.wd = "/srv/repo/pkg"
	f.files["/srv/repo/MODULE.bazel"] = ""
	f.owners["/srv/repo/MODULE.bazel"] = 2000
	f.vars[EnvTrustedConfigDirs] = "/missing"
	_, warnings, err := Load(f.env(), Layer{Kind: LayerFlag})
	require.NoError(t, err)
	require.Len(t, warnings, 2)
	assert.Contains(t, warnings[0], `"/missing" does not exist`)
	assert.Contains(t, warnings[1], "workspace marker /srv/repo/MODULE.bazel is owned by uid 2000")
}

func TestLoad_ErrorBeforePathAndWarningsStillReturned(t *testing.T) {
	f := newFakeEnv()
	f.files[projectFile] = ""
	f.owners[projectFile] = 2000
	f.vars[EnvTrustedConfigDirs] = "/missing"
	eff, warnings, err := Load(f.env(), Layer{Kind: LayerFlag})
	require.ErrorIs(t, err, ErrUntrustedConfig)
	assert.Nil(t, eff)
	assert.Len(t, warnings, 1)
}

func TestMessagesRenderHostilePaths(t *testing.T) {
	const bad = "/w\nz\x1b[31m\xff/.bz.toml"
	const shown = `"/w\nz\x1b[31m\xff/.bz.toml"`

	assert.Equal(t, "project config "+shown, Source{Layer: LayerProject, Origin: bad}.String())

	_, err := ParseFile(LayerProject, bad, []byte("[network\n"), "")
	require.EqualError(t, err, "parsing project config "+shown+": invalid TOML at line 1, column 9: invalid table name")

	m := NetworkMode("bogus")
	_, err = Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: bad, Settings: Settings{Mode: &m}})
	require.EqualError(t, err, `invalid network mode "bogus" in project config `+shown+` (want "online", "prefer-offline" or "offline")`)
}

// TestLoad_RealFilesPrecedence runs the loader on real temporary files
// through the OS environment (isolated from the real home and /etc). The
// system layer is covered by the fake-environment matrix, because a real
// system file must be owned by root.
func TestLoad_RealFilesPrecedence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ownership rules differ on Windows")
	}
	configHome := t.TempDir()
	ws := t.TempDir()
	userPath := filepath.Join(configHome, "bz", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(userPath), 0o700))
	require.NoError(t, os.WriteFile(userPath, []byte("[network]\nmode = \"offline\"\nregistry = \"https://user.example\"\ntimeout = \"5s\"\n[cache]\nttl = \"1h\"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(ws, "MODULE.bazel"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(ws, ".bz.toml"), []byte("[network]\nregistry = \"https://project.example\"\ntimeout = \"30s\"\n"), 0o600))
	t.Chdir(ws)

	env := OSEnvironment().Isolated(configHome).WithEnv(map[string]string{
		EnvCacheDir: "/from/env", EnvOffline: "", EnvPreferOffline: "", EnvRegistry: "", EnvDisableCommands: "", EnvTrustedConfigDirs: "",
	})
	eff, warnings, err := Load(env, FlagLayer(false, true, ""))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Equal(t, NetworkModePreferOffline, eff.Network.Mode)
	assert.Equal(t, LayerFlag, eff.Source(KeyMode).Layer)
	assert.Equal(t, "https://project.example", eff.Network.Registry)
	assert.Equal(t, LayerProject, eff.Source(KeyRegistry).Layer)
	assert.Equal(t, DefaultTimeout, eff.Network.Timeout, "project sets the default back")
	assert.Equal(t, LayerProject, eff.Source(KeyTimeout).Layer)
	assert.Equal(t, "/from/env", eff.Cache.Dir)
	assert.Equal(t, LayerEnv, eff.Source(KeyCacheDir).Layer)
	assert.Equal(t, "1h", eff.Cache.TTL)
	assert.Equal(t, Source{Layer: LayerUser, Origin: userPath}, eff.Source(KeyCacheTTL))
}
