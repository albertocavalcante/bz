package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func TestConfigDefaults(t *testing.T) {
	t.Run("basic defaults with registry", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)

		configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
		registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
		globals := starlark.StringDict{
			"config":   configModule,
			"registry": registryModule,
		}

		code := `
bcr = registry.http("https://bcr.bazel.build")
config.defaults(registry = bcr)
`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.NoError(t, err)

		defaults := GetConfigDefaults(thread)
		require.NotNil(t, defaults)
		require.NotNil(t, defaults.Registry)
		assert.Equal(t, RegistryTypeHTTP, defaults.Registry.Kind)
		assert.Equal(t, "https://bcr.bazel.build", defaults.Registry.URL)
	})

	t.Run("defaults with cache_dir", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)

		configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
		globals := starlark.StringDict{"config": configModule}

		code := `config.defaults(cache_dir = "~/.cache/bz")`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.NoError(t, err)

		defaults := GetConfigDefaults(thread)
		require.NotNil(t, defaults)
		assert.Equal(t, "~/.cache/bz", defaults.CacheDir)
	})

	t.Run("defaults with fallback_registries", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)

		configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
		registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
		globals := starlark.StringDict{
			"config":   configModule,
			"registry": registryModule,
		}

		code := `
bcr = registry.http("https://bcr.bazel.build")
mirror = registry.http("https://mirror.example.com")
local = registry.file("/path/to/local")

config.defaults(
    registry = bcr,
    fallback_registries = [mirror, local],
)
`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.NoError(t, err)

		defaults := GetConfigDefaults(thread)
		require.NotNil(t, defaults)
		require.NotNil(t, defaults.Registry)
		assert.Len(t, defaults.FallbackRegistries, 2)
		assert.Equal(t, RegistryTypeHTTP, defaults.FallbackRegistries[0].Kind)
		assert.Equal(t, "https://mirror.example.com", defaults.FallbackRegistries[0].URL)
		assert.Equal(t, RegistryTypeFile, defaults.FallbackRegistries[1].Kind)
		assert.Equal(t, "/path/to/local", defaults.FallbackRegistries[1].URL)
	})

	t.Run("defaults with all options", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)

		configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
		registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
		globals := starlark.StringDict{
			"config":   configModule,
			"registry": registryModule,
		}

		code := `
bcr = registry.http("https://bcr.bazel.build")
mirror = registry.http("https://mirror.example.com")

config.defaults(
    registry = bcr,
    cache_dir = "~/.cache/bz",
    fallback_registries = [mirror],
)
`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.NoError(t, err)

		defaults := GetConfigDefaults(thread)
		require.NotNil(t, defaults)
		assert.NotNil(t, defaults.Registry)
		assert.Equal(t, "~/.cache/bz", defaults.CacheDir)
		assert.Len(t, defaults.FallbackRegistries, 1)
	})

	t.Run("empty defaults", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)

		configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
		globals := starlark.StringDict{"config": configModule}

		code := `config.defaults()`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.NoError(t, err)

		defaults := GetConfigDefaults(thread)
		require.NotNil(t, defaults)
		assert.Nil(t, defaults.Registry)
		assert.Empty(t, defaults.CacheDir)
		assert.Nil(t, defaults.FallbackRegistries)
	})

	t.Run("invalid registry type", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)

		configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
		globals := starlark.StringDict{"config": configModule}

		code := `config.defaults(registry = "not a registry")`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry must be a registry value")
	})

	t.Run("invalid fallback_registries type", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)

		configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
		globals := starlark.StringDict{"config": configModule}

		code := `config.defaults(fallback_registries = "not a list")`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fallback_registries must be a list")
	})

	t.Run("invalid fallback_registries element", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)

		configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
		globals := starlark.StringDict{"config": configModule}

		code := `config.defaults(fallback_registries = ["not a registry"])`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fallback_registries[0] must be a registry value")
	})
}

func TestConfigDefaultsStorage(t *testing.T) {
	t.Run("get defaults when not initialized", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		defaults := GetConfigDefaults(thread)
		assert.Nil(t, defaults)
	})

	t.Run("get defaults when initialized but not set", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)
		defaults := GetConfigDefaults(thread)
		assert.Nil(t, defaults) // Initialized but not yet populated by config.defaults()
	})

	t.Run("defaults value updated by subsequent calls", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetConfigDefaults(thread)

		configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
		registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
		globals := starlark.StringDict{
			"config":   configModule,
			"registry": registryModule,
		}

		// First call
		code1 := `config.defaults(cache_dir = "first")`
		_, err := starlark.ExecFile(thread, "test1.star", code1, globals)
		require.NoError(t, err)

		defaults1 := GetConfigDefaults(thread)
		assert.Equal(t, "first", defaults1.CacheDir)

		// Second call overwrites
		code2 := `config.defaults(cache_dir = "second")`
		_, err = starlark.ExecFile(thread, "test2.star", code2, globals)
		require.NoError(t, err)

		defaults2 := GetConfigDefaults(thread)
		assert.Equal(t, "second", defaults2.CacheDir)
	})
}

func TestDefaultsValueString(t *testing.T) {
	t.Run("with registry", func(t *testing.T) {
		d := &DefaultsValue{
			Registry: &RegistryValue{
				Kind: RegistryTypeHTTP,
				URL:  "https://example.com",
			},
		}
		assert.Equal(t, `config.defaults(registry=registry.http("https://example.com"))`, d.String())
	})

	t.Run("without registry", func(t *testing.T) {
		d := &DefaultsValue{}
		assert.Equal(t, "config.defaults()", d.String())
	})
}

func TestDefaultsValueAttrs(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	SetConfigDefaults(thread)

	configModule := starlarkstruct.FromStringDict(starlark.String("config"), ConfigModule())
	registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
	globals := starlark.StringDict{
		"config":   configModule,
		"registry": registryModule,
	}

	code := `
bcr = registry.http("https://bcr.bazel.build")
d = config.defaults(
    registry = bcr,
    cache_dir = "~/.cache/bz",
)
`
	result, err := starlark.ExecFile(thread, "test.star", code, globals)
	require.NoError(t, err)

	// Get the defaults value from the result
	dVal := result["d"]
	require.NotNil(t, dVal)

	defaults, ok := dVal.(*DefaultsValue)
	require.True(t, ok)

	// Test attribute access
	registryAttr, err := defaults.Attr("registry")
	require.NoError(t, err)
	assert.NotNil(t, registryAttr)

	cacheDirAttr, err := defaults.Attr("cache_dir")
	require.NoError(t, err)
	assert.Equal(t, starlark.String("~/.cache/bz"), cacheDirAttr)

	// Test AttrNames
	names := defaults.AttrNames()
	assert.Contains(t, names, "registry")
	assert.Contains(t, names, "cache_dir")
}

func TestConfigModule(t *testing.T) {
	module := ConfigModule()
	assert.Contains(t, module, "defaults")
}
