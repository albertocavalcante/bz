package bzconfig

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setting describes how one setting is written in each layer, for the
// precedence matrix.
type setting struct {
	key Key
	// low, high and def are values in rendered string form; def is the
	// built-in default.
	low, high, def string
	toml           func(v string) string
	// env returns the variables that set v, or nil when the environment
	// cannot express v.
	env func(v string) map[string]string
	// flag returns the flag layer setting v, or false when flags cannot.
	flag func(v string) (Layer, bool)
	get  func(e *Effective) string
}

func quotedList(v string) string {
	if v == "" {
		return "[]"
	}
	parts := strings.Split(v, ",")
	for i, p := range parts {
		parts[i] = fmt.Sprintf("%q", p)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func modeEnv(v string) map[string]string {
	switch NetworkMode(v) {
	case NetworkModeOffline:
		return map[string]string{EnvOffline: "1"}
	case NetworkModePreferOffline:
		return map[string]string{EnvPreferOffline: "true"}
	default:
		return nil // the environment cannot select "online"
	}
}

func modeFlag(v string) (Layer, bool) {
	switch NetworkMode(v) {
	case NetworkModeOffline:
		return FlagLayer(true, false, ""), true
	case NetworkModePreferOffline:
		return FlagLayer(false, true, ""), true
	default:
		return Layer{}, false
	}
}

const defaultTestCacheDir = "/home/u/.cache/bz"

func settings() []setting {
	return []setting{
		{
			key: KeyMode, low: "offline", high: "prefer-offline", def: "online",
			toml: func(v string) string { return fmt.Sprintf("[network]\nmode = %q\n", v) },
			env:  modeEnv,
			flag: modeFlag,
			get:  func(e *Effective) string { return string(e.Network.Mode) },
		},
		{
			key: KeyRegistry, low: "https://low.example", high: "/srv/registry", def: DefaultRegistry,
			toml: func(v string) string { return fmt.Sprintf("[network]\nregistry = %q\n", v) },
			env:  func(v string) map[string]string { return map[string]string{EnvRegistry: v} },
			flag: func(v string) (Layer, bool) { return FlagLayer(false, false, v), true },
			get:  func(e *Effective) string { return e.Network.Registry },
		},
		{
			key: KeyFallbackRegistries, low: "https://a.example", high: "https://b.example,https://c.example", def: "",
			toml: func(v string) string { return fmt.Sprintf("[network]\nfallback_registries = %s\n", quotedList(v)) },
			get:  func(e *Effective) string { return strings.Join(e.Network.FallbackRegistries, ",") },
		},
		{
			key: KeyTimeout, low: "5s", high: "2m", def: DefaultTimeout,
			toml: func(v string) string { return fmt.Sprintf("[network]\ntimeout = %q\n", v) },
			get:  func(e *Effective) string { return e.Network.Timeout },
		},
		{
			key: KeyCacheDir, low: "/low/cache", high: "/high/cache", def: defaultTestCacheDir,
			toml: func(v string) string { return fmt.Sprintf("[cache]\ndir = %q\n", v) },
			env:  func(v string) map[string]string { return map[string]string{EnvCacheDir: v} },
			get:  func(e *Effective) string { return e.Cache.Dir },
		},
		{
			key: KeyCacheTTL, low: "1h", high: "24h", def: DefaultTTL,
			toml: func(v string) string { return fmt.Sprintf("[cache]\nttl = %q\n", v) },
			get:  func(e *Effective) string { return e.Cache.TTL },
		},
		{
			key: KeyDisabled, low: "audit", high: "sbom,registry ping", def: "",
			toml: func(v string) string { return fmt.Sprintf("[commands]\ndisabled = %s\n", quotedList(v)) },
			env: func(v string) map[string]string {
				if v == "" {
					return map[string]string{EnvDisableCommands: ","} // separators only clear the list
				}
				return map[string]string{EnvDisableCommands: v}
			},
			get: func(e *Effective) string { return strings.Join(e.Commands.Disabled, ",") },
		},
	}
}

var fileLayers = map[LayerKind]string{LayerSystem: systemFile, LayerUser: userFile, LayerProject: projectFile}

// put writes value v of s into layer kind, returning the flag layer when
// kind is LayerFlag. ok is false when the layer cannot express v.
func put(f *fakeEnv, s setting, kind LayerKind, v string) (flags Layer, ok bool) {
	switch kind {
	case LayerSystem, LayerUser, LayerProject:
		f.files[fileLayers[kind]] += s.toml(v)
		return Layer{Kind: LayerFlag}, true
	case LayerEnv:
		if s.env == nil {
			return Layer{}, false
		}
		vars := s.env(v)
		if vars == nil {
			return Layer{}, false
		}
		for k, val := range vars {
			f.vars[k] = val
		}
		return Layer{Kind: LayerFlag}, true
	case LayerFlag:
		if s.flag == nil {
			return Layer{}, false
		}
		return s.flag(v)
	default:
		return Layer{}, false
	}
}

// TestLoad_PrecedenceMatrix checks every setting against every layer: a
// layer overrides all lower layers, and can set the value back to the
// built-in default.
func TestLoad_PrecedenceMatrix(t *testing.T) {
	targets := []LayerKind{LayerSystem, LayerUser, LayerProject, LayerEnv, LayerFlag}
	for _, s := range settings() {
		for _, target := range targets {
			for _, back := range []bool{false, true} {
				want := s.high
				name := fmt.Sprintf("%s/%s/overrides", s.key, target)
				if back {
					want = s.def
					name = fmt.Sprintf("%s/%s/sets-default", s.key, target)
				}
				t.Run(name, func(t *testing.T) {
					f := newFakeEnv()
					// Every layer below target sets the low value.
					for _, lower := range targets {
						if lower >= target {
							break
						}
						if lower == LayerFlag {
							continue
						}
						put(f, s, lower, s.low)
					}
					flags, ok := put(f, s, target, want)
					if !ok {
						t.Skipf("%s cannot express %q", target, want)
					}
					eff := mustLoad(t, f, flags)
					assert.Equal(t, want, s.get(eff))
					assert.Equal(t, target, eff.Source(s.key).Layer, "source of %s", s.key)
				})
			}
		}
	}
}

func TestLoad_DefaultsWhenNothingSet(t *testing.T) {
	f := newFakeEnv()
	eff := mustLoad(t, f, Layer{Kind: LayerFlag})
	for _, s := range settings() {
		assert.Equal(t, s.def, s.get(eff), "%s", s.key)
		assert.Equal(t, LayerDefault, eff.Source(s.key).Layer, "%s", s.key)
	}
	assert.Equal(t, "default", eff.Source(KeyMode).String())
}

// Regression: every file used to carry mode=online after per-file
// defaults, so a project file that only set [cache] dir switched a user's
// offline mode back to online.
func TestLoad_LowerLayerSurvivesUnrelatedHigherFile(t *testing.T) {
	f := newFakeEnv()
	f.files[userFile] = "[network]\nmode = \"offline\"\n"
	f.files[projectFile] = "[cache]\ndir = \"/project/cache\"\n"

	eff := mustLoad(t, f, Layer{Kind: LayerFlag})
	assert.Equal(t, NetworkModeOffline, eff.Network.Mode)
	assert.Equal(t, Source{Layer: LayerUser, Origin: userFile}, eff.Source(KeyMode))
	assert.Equal(t, "/project/cache", eff.Cache.Dir)
	assert.Equal(t, Source{Layer: LayerProject, Origin: projectFile}, eff.Source(KeyCacheDir))
	assert.Equal(t, []string{userFile, projectFile}, eff.Files)
}

// Regression: merge skipped values equal to the default, so a project file
// could not set the registry or timeout back to the default.
func TestLoad_HigherLayerSetsValueBackToDefault(t *testing.T) {
	f := newFakeEnv()
	f.files[userFile] = "[network]\nregistry = \"https://mirror.example\"\ntimeout = \"5s\"\nmode = \"offline\"\n"
	f.files[projectFile] = fmt.Sprintf("[network]\nregistry = %q\ntimeout = %q\nmode = \"online\"\n", DefaultRegistry, DefaultTimeout)

	eff := mustLoad(t, f, Layer{Kind: LayerFlag})
	assert.Equal(t, DefaultRegistry, eff.Network.Registry)
	assert.Equal(t, DefaultTimeout, eff.Network.Timeout)
	assert.Equal(t, NetworkModeOnline, eff.Network.Mode)
	for _, k := range []Key{KeyRegistry, KeyTimeout, KeyMode} {
		assert.Equal(t, LayerProject, eff.Source(k).Layer, "%s", k)
	}
}

func TestLoad_FlagModeBeatsConfigAndEnvMode(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		vars       map[string]string
		flags      Layer
		want       NetworkMode
		wantOrigin string
	}{
		{
			name:  "config offline, --prefer-offline",
			file:  "[network]\nmode = \"offline\"\n",
			flags: FlagLayer(false, true, ""), want: NetworkModePreferOffline, wantOrigin: "--prefer-offline",
		},
		{
			name:  "config prefer-offline, --offline",
			file:  "[network]\nmode = \"prefer-offline\"\n",
			flags: FlagLayer(true, false, ""), want: NetworkModeOffline, wantOrigin: "--offline",
		},
		{
			name: "BZ_OFFLINE, --prefer-offline",
			vars: map[string]string{EnvOffline: "1"}, flags: FlagLayer(false, true, ""),
			want: NetworkModePreferOffline, wantOrigin: "--prefer-offline",
		},
		{
			name: "BZ_OFFLINE wins over BZ_PREFER_OFFLINE",
			vars: map[string]string{EnvOffline: "1", EnvPreferOffline: "1"}, flags: Layer{Kind: LayerFlag},
			want: NetworkModeOffline, wantOrigin: EnvOffline,
		},
		{
			name:  "BZ_OFFLINE=0 leaves config offline in force",
			file:  "[network]\nmode = \"offline\"\n",
			vars:  map[string]string{EnvOffline: "0"},
			flags: Layer{Kind: LayerFlag}, want: NetworkModeOffline, wantOrigin: projectFile,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			if tt.file != "" {
				f.files[projectFile] = tt.file
			}
			for k, v := range tt.vars {
				f.vars[k] = v
			}
			eff := mustLoad(t, f, tt.flags)
			assert.Equal(t, tt.want, eff.Network.Mode)
			assert.Equal(t, tt.wantOrigin, eff.Source(KeyMode).Origin)
		})
	}
}

func TestLoad_InvalidValuesNameTheirSource(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		vars    map[string]string
		wantSub []string
	}{
		{name: "mode", file: "[network]\nmode = \"ofline\"\n", wantSub: []string{"ofline", "user config " + userFile}},
		{name: "timeout", file: "[network]\ntimeout = \"thirty\"\n", wantSub: []string{"thirty", userFile}},
		{name: "negative timeout", file: "[network]\ntimeout = \"-1s\"\n", wantSub: []string{"-1s"}},
		{name: "empty registry", file: "[network]\nregistry = \"\"\n", wantSub: []string{"empty registry", userFile}},
		{name: "wrong type", file: "[network]\nmode = 1\n", wantSub: []string{userFile}},
		{name: "malformed", file: "[network\n", wantSub: []string{userFile}},
		{name: "bad bool env", vars: map[string]string{EnvOffline: "ture"}, wantSub: []string{EnvOffline, "ture"}},
		{name: "timeout without unit", file: "[network]\ntimeout = \"30\"\n", wantSub: []string{`"30"`}},
		{name: "relative registry", file: "[network]\nregistry = \"foo\"\n", wantSub: []string{`"foo"`, userFile}},
		{name: "registry without host", file: "[network]\nregistry = \"https://\"\n", wantSub: []string{"missing host"}},
		{name: "bad registry env", vars: map[string]string{EnvRegistry: "bcr.example"}, wantSub: []string{"environment BZ_REGISTRY"}},
		{name: "bad fallback entry", file: "[network]\nfallback_registries = [\"https://ok.example\", \"mirror\"]\n", wantSub: []string{"fallback_registries", `"mirror"`}},
		{name: "ttl", file: "[cache]\nttl = \"7d\"\n", wantSub: []string{`invalid cache ttl "7d"`, userFile}},
		{name: "negative ttl", file: "[cache]\nttl = \"-1h\"\n", wantSub: []string{`"-1h"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeEnv()
			if tt.file != "" {
				f.files[userFile] = tt.file
			}
			for k, v := range tt.vars {
				f.vars[k] = v
			}
			_, _, err := Load(f.env(), Layer{Kind: LayerFlag})
			require.Error(t, err)
			for _, sub := range tt.wantSub {
				assert.Contains(t, err.Error(), sub)
			}
		})
	}
}

func TestLoad_AcceptedRegistryForms(t *testing.T) {
	for _, reg := range []string{"https://bcr.bazel.build", "http://internal:8080/bcr", "file:///opt/registry", "/opt/registry", `C:\registry`, "D:/registry"} {
		f := newFakeEnv()
		f.files[projectFile] = fmt.Sprintf("[network]\nregistry = %q\nfallback_registries = [%q]\n", reg, reg)
		eff := mustLoad(t, f, Layer{Kind: LayerFlag})
		assert.Equal(t, reg, eff.Network.Registry)
	}
}

func TestLoad_DisabledEntriesFromFilesAreTrimmed(t *testing.T) {
	f := newFakeEnv()
	f.files[projectFile] = "[commands]\ndisabled = [\" audit \", \"\", \"registry ping\"]\n"
	eff := mustLoad(t, f, Layer{Kind: LayerFlag})
	assert.Equal(t, []string{"audit", "registry ping"}, eff.Commands.Disabled)
}

func TestLoad_EmptyTTLIsAllowed(t *testing.T) {
	f := newFakeEnv()
	f.files[projectFile] = "[cache]\nttl = \"\"\n"
	eff := mustLoad(t, f, Layer{Kind: LayerFlag})
	assert.Equal(t, "", eff.Cache.TTL)
	assert.Equal(t, LayerProject, eff.Source(KeyCacheTTL).Layer)
}
