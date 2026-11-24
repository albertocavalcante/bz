package bzconfig

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parse(t *testing.T, kind LayerKind, path, data string) Layer {
	t.Helper()
	l, _, err := ParseFile(kind, path, []byte(data), "/home/u")
	require.NoError(t, err)
	return l
}

// Regression: every file used to carry mode=online after per-file
// defaults, so a project file that only set [cache] dir switched a user's
// offline mode back to online.
func TestMerge_LowerLayerSurvivesUnrelatedHigherLayer(t *testing.T) {
	user := parse(t, LayerUser, "/u/config.toml", "[network]\nmode = \"offline\"\n")
	project := parse(t, LayerProject, "/p/.bz.toml", "[cache]\ndir = \"/project/cache\"\n")

	eff, err := Merge(Defaults("/default/cache"), user, project)
	require.NoError(t, err)
	assert.Equal(t, NetworkModeOffline, eff.Network.Mode)
	assert.Equal(t, Source{Layer: LayerUser, Origin: "/u/config.toml"}, eff.Source(KeyMode))
	assert.Equal(t, "/project/cache", eff.Cache.Dir)
	assert.Equal(t, LayerProject, eff.Source(KeyCacheDir).Layer)
	assert.Equal(t, DefaultRegistry, eff.Network.Registry)
	assert.Equal(t, LayerDefault, eff.Source(KeyRegistry).Layer)
}

// Regression: merge skipped values equal to the default, so a higher layer
// could not set a value back to the default.
func TestMerge_HigherLayerSetsValueBackToDefault(t *testing.T) {
	user := parse(t, LayerUser, "/u/config.toml",
		"[network]\nmode = \"offline\"\nregistry = \"https://mirror.example\"\ntimeout = \"5s\"\n"+
			"[cache]\ndir = \"/custom\"\n[commands]\ndisabled = [\"audit\"]\n")
	project := parse(t, LayerProject, "/p/.bz.toml",
		"[network]\nmode = \"online\"\nregistry = \"https://bcr.bazel.build\"\ntimeout = \"30s\"\n"+
			"[cache]\ndir = \"/default/cache\"\n[commands]\ndisabled = []\n")

	eff, err := Merge(Defaults("/default/cache"), user, project)
	require.NoError(t, err)
	assert.Equal(t, NetworkModeOnline, eff.Network.Mode)
	assert.Equal(t, DefaultRegistry, eff.Network.Registry)
	assert.Equal(t, DefaultTimeout, eff.Network.Timeout)
	assert.Equal(t, "/default/cache", eff.Cache.Dir)
	assert.Empty(t, eff.Commands.Disabled)
	for _, k := range []Key{KeyMode, KeyRegistry, KeyTimeout, KeyCacheDir, KeyDisabled} {
		assert.Equal(t, LayerProject, eff.Source(k).Layer, "%s", k)
	}
}

func TestMerge_FlagModeBeatsLowerLayers(t *testing.T) {
	offline := parse(t, LayerProject, "/p/.bz.toml", "[network]\nmode = \"offline\"\n")
	eff, err := Merge(Defaults("/c"), offline, FlagLayer(false, true, ""))
	require.NoError(t, err)
	assert.Equal(t, NetworkModePreferOffline, eff.Network.Mode)
	assert.Equal(t, "flag --prefer-offline", eff.Source(KeyMode).String())

	prefer := parse(t, LayerProject, "/p/.bz.toml", "[network]\nmode = \"prefer-offline\"\n")
	eff, err = Merge(Defaults("/c"), prefer, FlagLayer(true, false, "https://flag.example"))
	require.NoError(t, err)
	assert.Equal(t, NetworkModeOffline, eff.Network.Mode)
	assert.Equal(t, "https://flag.example", eff.Network.Registry)
	assert.Equal(t, "flag --registry", eff.Source(KeyRegistry).String())
}

func TestMerge_InvalidValuesNameTheirSource(t *testing.T) {
	tests := []struct {
		name, data, want string
	}{
		{name: "mode", data: "[network]\nmode = \"ofline\"\n", want: `invalid network mode "ofline" in project config /p/.bz.toml`},
		{name: "timeout", data: "[network]\ntimeout = \"thirty\"\n", want: `invalid network timeout "thirty" in project config /p/.bz.toml`},
		{name: "negative timeout", data: "[network]\ntimeout = \"-1s\"\n", want: `"-1s"`},
		{name: "empty registry", data: "[network]\nregistry = \"\"\n", want: "empty registry in project config /p/.bz.toml"},
		{name: "empty cache dir", data: "[cache]\ndir = \"\"\n", want: "empty cache dir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Merge(Defaults("/c"), parse(t, LayerProject, "/p/.bz.toml", tt.data))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestParseFile_Errors(t *testing.T) {
	for _, data := range []string{"[network\n", "[network]\nmode = 1\n"} {
		_, _, err := ParseFile(LayerUser, "/u/config.toml", []byte(data), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/u/config.toml")
	}
}

func TestParseFile_CacheDirExpansion(t *testing.T) {
	l := parse(t, LayerProject, "/ws/.bz.toml", "[cache]\ndir = \"~/c\"\n")
	assert.Equal(t, "/home/u/c", *l.Settings.CacheDir)
	l = parse(t, LayerProject, "/ws/.bz.toml", "[cache]\ndir = \"rel\"\n")
	assert.Equal(t, "/ws/rel", *l.Settings.CacheDir)
}

func TestExpandHome(t *testing.T) {
	assert.Equal(t, "/home/u", ExpandHome("~", "/home/u"))
	assert.Equal(t, "/home/u/x/y", ExpandHome("~/x/y", "/home/u"))
	assert.Equal(t, "~other/x", ExpandHome("~other/x", "/home/u"))
	assert.Equal(t, "/abs", ExpandHome("/abs", "/home/u"))
	assert.Equal(t, "~/x", ExpandHome("~/x", ""))
}

func TestEnvLayer(t *testing.T) {
	lookup := func(vars map[string]string) func(string) (string, bool) {
		return func(n string) (string, bool) { v, ok := vars[n]; return v, ok }
	}

	t.Run("empty values are unset", func(t *testing.T) {
		l, err := EnvLayer(lookup(map[string]string{
			EnvOffline: "", EnvRegistry: " ", EnvCacheDir: "", EnvDisableCommands: " , ",
		}), "/home/u")
		require.NoError(t, err)
		assert.Equal(t, Settings{}, l.Settings)
	})

	t.Run("values and origins", func(t *testing.T) {
		l, err := EnvLayer(lookup(map[string]string{
			EnvPreferOffline: "YES", EnvRegistry: "https://r.example",
			EnvCacheDir: "~/c", EnvDisableCommands: "audit, registry ping",
		}), "/home/u")
		require.NoError(t, err)
		require.NotNil(t, l.Settings.Mode)
		assert.Equal(t, NetworkModePreferOffline, *l.Settings.Mode)
		assert.Equal(t, "/home/u/c", *l.Settings.CacheDir)
		assert.Equal(t, []string{"audit", "registry ping"}, *l.Settings.Disabled)
		assert.Equal(t, "environment BZ_CACHE_DIR", l.source(KeyCacheDir).String())
		assert.Equal(t, "environment BZ_PREFER_OFFLINE", l.source(KeyMode).String())
	})

	t.Run("invalid boolean", func(t *testing.T) {
		_, err := EnvLayer(lookup(map[string]string{EnvPreferOffline: "maybe"}), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), EnvPreferOffline)
	})
}

func TestEnvVarsListIsComplete(t *testing.T) {
	assert.Equal(t, []string{"BZ_OFFLINE", "BZ_PREFER_OFFLINE", "BZ_REGISTRY", "BZ_CACHE_DIR", "BZ_DISABLE_COMMANDS"}, EnvVars())
}

func TestMerge_DoesNotAliasLayerSlices(t *testing.T) {
	list := []string{"audit"}
	eff, err := Merge(Defaults("/c"), Layer{Kind: LayerUser, Settings: Settings{Disabled: &list}})
	require.NoError(t, err)
	eff.Commands.Disabled[0] = "changed"
	assert.Equal(t, "audit", list[0])
	assert.True(t, slices.Equal([]string{"changed"}, eff.Commands.Disabled))
}
