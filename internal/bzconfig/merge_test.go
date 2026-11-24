package bzconfig

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parse(t *testing.T, kind LayerKind, path, data string) Layer {
	t.Helper()
	l, err := ParseFile(kind, path, []byte(data), "/home/u")
	require.NoError(t, err)
	return l
}

func lookupFrom(vars map[string]string) func(string) (string, bool) {
	return func(n string) (string, bool) { v, ok := vars[n]; return v, ok }
}

func envLayer(t *testing.T, vars map[string]string) Layer {
	t.Helper()
	l, err := EnvLayer(lookupFrom(vars), "/home/u")
	require.NoError(t, err)
	return l
}

// Regression: every file used to carry mode=online after per-file
// defaults, so a higher layer that did not mention the mode switched a
// lower layer's offline mode back to online.
func TestMerge_LowerLayerSurvivesUnrelatedHigherLayer(t *testing.T) {
	project := parse(t, LayerProject, "/p/.bzconfig.toml", "[network]\nmode = \"offline\"\n")
	env := envLayer(t, map[string]string{EnvCacheDir: "/env/cache"})

	eff, err := Merge(Defaults("/default/cache"), project, env)
	require.NoError(t, err)
	assert.Equal(t, NetworkModeOffline, eff.Network.Mode)
	assert.Equal(t, Source{Layer: LayerProject, Origin: "/p/.bzconfig.toml"}, eff.Source(KeyMode))
	assert.Equal(t, "/env/cache", eff.Cache.Dir)
	assert.Equal(t, Source{Layer: LayerEnv, Origin: EnvCacheDir}, eff.Source(KeyCacheDir))
	assert.Equal(t, DefaultRegistry, eff.Network.Registry)
	assert.Equal(t, Source{Layer: LayerDefault}, eff.Source(KeyRegistry))
}

// Regression: merge skipped values equal to the default, so a higher layer
// could not set a value back to the default.
func TestMerge_HigherLayerSetsValueBackToDefault(t *testing.T) {
	const lowPath, highPath = "/low/.bzconfig.toml", "/high/.bzconfig.toml"
	low := parse(t, LayerProject, lowPath,
		"[network]\nmode = \"offline\"\nregistry = \"https://mirror.example\"\ntimeout = \"5s\"\n"+
			"[cache]\ndir = \"/custom\"\n[commands]\ndisabled = [\"audit\"]\n")
	high := parse(t, LayerProject, highPath,
		"[network]\nmode = \"online\"\nregistry = \"https://bcr.bazel.build\"\ntimeout = \"30s\"\n"+
			"[cache]\ndir = \"/default/cache\"\n[commands]\ndisabled = []\n")

	eff, err := Merge(Defaults("/default/cache"), low, high)
	require.NoError(t, err)
	assert.Equal(t, NetworkModeOnline, eff.Network.Mode)
	assert.Equal(t, DefaultRegistry, eff.Network.Registry)
	assert.Equal(t, DefaultTimeout, eff.Network.Timeout)
	assert.Equal(t, "/default/cache", eff.Cache.Dir)
	assert.Empty(t, eff.Commands.Disabled)
	want := Source{Layer: LayerProject, Origin: highPath}
	for _, k := range []Key{KeyMode, KeyRegistry, KeyTimeout, KeyCacheDir, KeyDisabled} {
		assert.Equal(t, want, eff.Source(k), "%s", k)
	}
}

func TestMerge_EnvBeatsProject(t *testing.T) {
	project := parse(t, LayerProject, "/p/.bzconfig.toml",
		"[network]\nmode = \"prefer-offline\"\nregistry = \"https://file.example\"\n")
	env := envLayer(t, map[string]string{EnvOffline: "1", EnvRegistry: "https://env.example"})

	eff, err := Merge(Defaults("/c"), project, env)
	require.NoError(t, err)
	assert.Equal(t, NetworkModeOffline, eff.Network.Mode)
	assert.Equal(t, "environment BZ_OFFLINE", eff.Source(KeyMode).String())
	assert.Equal(t, "https://env.example", eff.Network.Registry)
	assert.Equal(t, "environment BZ_REGISTRY", eff.Source(KeyRegistry).String())
}

// TestMerge_InvalidValuesAreSkippedAndReported: an invalid value keeps the
// lower layer's value, and the error names the value and its source.
func TestMerge_InvalidValuesAreSkippedAndReported(t *testing.T) {
	tests := []struct {
		name, data, want string
		check            func(t *testing.T, e *Effective)
	}{
		{
			name: "mode", data: "[network]\nmode = \"ofline\"\n",
			want:  `invalid network mode "ofline" in project config /p/.bzconfig.toml`,
			check: func(t *testing.T, e *Effective) { assert.Equal(t, NetworkModeOnline, e.Network.Mode) },
		},
		{
			name: "empty mode", data: "[network]\nmode = \"\"\n", want: `invalid network mode ""`,
			check: func(t *testing.T, e *Effective) { assert.Equal(t, NetworkModeOnline, e.Network.Mode) },
		},
		{
			name: "timeout", data: "[network]\ntimeout = \"thirty\"\n",
			want:  `invalid network timeout "thirty" in project config /p/.bzconfig.toml`,
			check: func(t *testing.T, e *Effective) { assert.Equal(t, DefaultTimeout, e.Network.Timeout) },
		},
		{name: "timeout without unit", data: "[network]\ntimeout = \"30\"\n", want: `"30"`},
		{name: "zero timeout", data: "[network]\ntimeout = \"0s\"\n", want: `"0s"`},
		{
			name: "empty registry", data: "[network]\nregistry = \"\"\n", want: "invalid registry in project config /p/.bzconfig.toml: empty registry URL",
			check: func(t *testing.T, e *Effective) { assert.Equal(t, DefaultRegistry, e.Network.Registry) },
		},
		{
			name: "empty cache dir", data: "[cache]\ndir = \"\"\n", want: "empty cache dir",
			check: func(t *testing.T, e *Effective) { assert.Equal(t, "/c", e.Cache.Dir) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eff, err := Merge(Defaults("/c"), parse(t, LayerProject, "/p/.bzconfig.toml", tt.data+"[commands]\ndisabled = [\"audit\"]\n"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			require.NotNil(t, eff)
			assert.Equal(t, []string{"audit"}, eff.Commands.Disabled, "valid keys still apply")
			if tt.check != nil {
				tt.check(t, eff)
			}
		})
	}
}

func TestParseFile_CompleteFileDecodesEveryField(t *testing.T) {
	l := parse(t, LayerProject, "/p/.bzconfig.toml", `
[network]
mode = "prefer-offline"
registry = "https://internal-registry.example.com"
fallback_registries = ["https://bcr.bazel.build"]
timeout = "60s"

[cache]
dir = "/custom/cache/path"
ttl = "24h"

[commands]
disabled = ["audit", "search"]
`)
	eff, err := Merge(Defaults("/c"), l)
	require.NoError(t, err)
	assert.Equal(t, NetworkModePreferOffline, eff.Network.Mode)
	assert.Equal(t, "https://internal-registry.example.com", eff.Network.Registry)
	assert.Equal(t, []string{"https://bcr.bazel.build"}, eff.Network.FallbackRegistries)
	assert.Equal(t, "60s", eff.Network.Timeout)
	assert.Equal(t, "/custom/cache/path", eff.Cache.Dir)
	assert.Equal(t, "24h", eff.Cache.TTL)
	assert.Equal(t, []string{"audit", "search"}, eff.Commands.Disabled)
}

func TestParseFile_Errors(t *testing.T) {
	for _, data := range []string{"[network\n", "this is not valid toml [[[", "[network]\nmode = 1\n"} {
		_, err := ParseFile(LayerProject, "/p/.bzconfig.toml", []byte(data), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/p/.bzconfig.toml")
	}
}

func TestParseFile_CacheDirTilde(t *testing.T) {
	l := parse(t, LayerProject, "/ws/.bzconfig.toml", "[cache]\ndir = \"~/c\"\n")
	assert.Equal(t, "/home/u/c", *l.Settings.CacheDir)
}

func TestExpandHome(t *testing.T) {
	assert.Equal(t, "/home/u", ExpandHome("~", "/home/u"))
	assert.Equal(t, "/home/u/x/y", ExpandHome("~/x/y", "/home/u"))
	assert.Equal(t, "~other/x", ExpandHome("~other/x", "/home/u"))
	assert.Equal(t, "/abs", ExpandHome("/abs", "/home/u"))
	assert.Equal(t, "~/x", ExpandHome("~/x", ""))
}

func TestEnvLayer(t *testing.T) {
	t.Run("empty values are unset", func(t *testing.T) {
		l := envLayer(t, map[string]string{EnvOffline: "", EnvRegistry: " ", EnvCacheDir: "", EnvDisableCommands: ""})
		assert.Equal(t, Settings{}, l.Settings)
	})

	t.Run("separators only clear the disabled list", func(t *testing.T) {
		for _, v := range []string{",", " ", " , "} {
			l := envLayer(t, map[string]string{EnvDisableCommands: v})
			require.NotNil(t, l.Settings.Disabled, "%q", v)
			assert.Empty(t, *l.Settings.Disabled, "%q", v)
		}
	})

	t.Run("values and origins", func(t *testing.T) {
		l := envLayer(t, map[string]string{
			EnvPreferOffline: "YES", EnvRegistry: " https://r.example ",
			EnvCacheDir: "~/c", EnvDisableCommands: "audit, registry ping",
		})
		require.NotNil(t, l.Settings.Mode)
		assert.Equal(t, NetworkModePreferOffline, *l.Settings.Mode)
		assert.Equal(t, "https://r.example", *l.Settings.Registry)
		assert.Equal(t, "/home/u/c", *l.Settings.CacheDir)
		assert.Equal(t, []string{"audit", "registry ping"}, *l.Settings.Disabled)
		assert.Equal(t, "environment BZ_CACHE_DIR", l.source(KeyCacheDir).String())
		assert.Equal(t, "environment BZ_PREFER_OFFLINE", l.source(KeyMode).String())
	})

	t.Run("BZ_OFFLINE wins over BZ_PREFER_OFFLINE", func(t *testing.T) {
		l := envLayer(t, map[string]string{EnvOffline: "true", EnvPreferOffline: "1"})
		assert.Equal(t, NetworkModeOffline, *l.Settings.Mode)
	})

	t.Run("invalid boolean is reported and read as false", func(t *testing.T) {
		l, err := EnvLayer(lookupFrom(map[string]string{EnvOffline: "ture", EnvRegistry: "https://r.example"}), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), EnvOffline)
		assert.Nil(t, l.Settings.Mode)
		assert.Equal(t, "https://r.example", *l.Settings.Registry, "other variables still apply")
	})
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{in: "audit", want: []string{"audit"}},
		{in: "audit,search", want: []string{"audit", "search"}},
		{in: " audit , search ", want: []string{"audit", "search"}},
		{in: "audit,,search,", want: []string{"audit", "search"}},
		{in: "\taudit\t,\nsearch", want: []string{"audit", "search"}},
		{in: "", want: []string{}},
		{in: " , ", want: []string{}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, parseCommaSeparated(tt.in), "%q", tt.in)
	}
}

func TestConfigModePredicates(t *testing.T) {
	for _, tt := range []struct {
		mode            NetworkMode
		offline, prefer bool
	}{
		{NetworkModeOnline, false, false},
		{NetworkModeOffline, true, false},
		{NetworkModePreferOffline, false, true},
	} {
		c := Config{Network: NetworkConfig{Mode: tt.mode}}
		assert.Equal(t, tt.offline, c.IsOffline(), tt.mode)
		assert.Equal(t, tt.prefer, c.IsPreferOffline(), tt.mode)
	}
}

func TestMerge_DoesNotAliasLayerSlices(t *testing.T) {
	list := []string{"audit"}
	eff, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Settings: Settings{Disabled: &list}})
	require.NoError(t, err)
	eff.Commands.Disabled[0] = "changed"
	assert.Equal(t, "audit", list[0])
	assert.True(t, slices.Equal([]string{"changed"}, eff.Commands.Disabled))
}

func clearBZEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{EnvOffline, EnvPreferOffline, EnvRegistry, EnvCacheDir, EnvDisableCommands} {
		t.Setenv(v, "")
	}
}

func TestLoadProjectFile(t *testing.T) {
	t.Run("nonexistent file gives defaults", func(t *testing.T) {
		clearBZEnv(t)
		eff, err := LoadProjectFile(filepath.Join(t.TempDir(), "missing.toml"))
		require.NoError(t, err)
		assert.Equal(t, NetworkModeOnline, eff.Network.Mode)
		assert.Equal(t, DefaultRegistry, eff.Network.Registry)
		assert.Empty(t, eff.Commands.Disabled)
	})

	t.Run("invalid values are ignored, valid keys apply", func(t *testing.T) {
		clearBZEnv(t)
		t.Setenv(EnvPreferOffline, "maybe")
		path := filepath.Join(t.TempDir(), ".bzconfig.toml")
		require.NoError(t, os.WriteFile(path, []byte(
			"[network]\nmode = \"ofline\"\ntimeout = \"30\"\n[commands]\ndisabled = [\"audit\"]\n"), 0o644))
		eff, err := LoadProjectFile(path)
		require.NoError(t, err)
		assert.Equal(t, []string{"audit"}, eff.Commands.Disabled)
		assert.Equal(t, NetworkModeOnline, eff.Network.Mode)
		assert.Equal(t, DefaultTimeout, eff.Network.Timeout)
	})

	t.Run("environment overrides the file", func(t *testing.T) {
		clearBZEnv(t)
		t.Setenv(EnvDisableCommands, "sbom")
		path := filepath.Join(t.TempDir(), ".bzconfig.toml")
		require.NoError(t, os.WriteFile(path, []byte("[commands]\ndisabled = [\"audit\"]\n"), 0o644))
		eff, err := LoadProjectFile(path)
		require.NoError(t, err)
		assert.Equal(t, []string{"sbom"}, eff.Commands.Disabled)
	})

	t.Run("malformed file is an error", func(t *testing.T) {
		clearBZEnv(t)
		path := filepath.Join(t.TempDir(), ".bzconfig.toml")
		require.NoError(t, os.WriteFile(path, []byte("[network\n"), 0o644))
		_, err := LoadProjectFile(path)
		require.Error(t, err)
	})
}
