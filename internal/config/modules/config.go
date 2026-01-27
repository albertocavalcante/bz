package modules

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/albertocavalcante/bz/internal/starlark/value"
)

// Thread-local key for config defaults.
const configDefaultsKey = "config_defaults"

// DefaultsValue represents configuration defaults in Starlark.
type DefaultsValue struct {
	value.Base
	value.AttrAccessor

	// Registry is the default registry to use.
	Registry *RegistryValue

	// CacheDir is the directory for caching module data.
	CacheDir string

	// FallbackRegistries is a list of registries to try if the default fails.
	FallbackRegistries []*RegistryValue
}

// Ensure DefaultsValue implements the required interfaces.
var (
	_ starlark.Value    = (*DefaultsValue)(nil)
	_ starlark.HasAttrs = (*DefaultsValue)(nil)
)

// String returns a string representation of the defaults.
func (d *DefaultsValue) String() string {
	if d.Registry != nil {
		return fmt.Sprintf("config.defaults(registry=%s)", d.Registry.String())
	}
	return "config.defaults()"
}

// SetConfigDefaults sets the config defaults on a Starlark thread.
// This initializes storage for defaults values.
func SetConfigDefaults(thread *starlark.Thread) {
	// Initialize with nil - will be set when config.defaults() is called
	thread.SetLocal(configDefaultsKey, (*DefaultsValue)(nil))
}

// GetConfigDefaults returns the configuration defaults set during Starlark execution.
func GetConfigDefaults(thread *starlark.Thread) *DefaultsValue {
	if v := thread.Local(configDefaultsKey); v != nil {
		if d, ok := v.(*DefaultsValue); ok {
			return d
		}
	}
	return nil
}

// setConfigDefaults stores the defaults in thread-local storage.
func setConfigDefaults(thread *starlark.Thread, defaults *DefaultsValue) {
	thread.SetLocal(configDefaultsKey, defaults)
}

// configDefaults creates a config.defaults configuration.
func configDefaults(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("config.defaults", args, kwargs)

	var registryVal, fallbackVal starlark.Value
	var cacheDir string

	if err := u.Optional("registry", &registryVal, starlark.None); err != nil {
		return nil, err
	}
	if err := u.Optional("cache_dir", &cacheDir, ""); err != nil {
		return nil, err
	}
	if err := u.Optional("fallback_registries", &fallbackVal, starlark.None); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	d := &DefaultsValue{
		Base:     value.NewBase("config_defaults"),
		CacheDir: cacheDir,
	}
	d.AttrAccessor = value.NewAttrAccessor()

	// Validate and set registry
	if registryVal != nil && registryVal != starlark.None {
		registry, ok := registryVal.(*RegistryValue)
		if !ok {
			return nil, fmt.Errorf("config.defaults: registry must be a registry value, got %s", registryVal.Type())
		}
		d.Registry = registry
		d.Set("registry", registry)
	}

	// Set cache_dir
	if cacheDir != "" {
		d.Set("cache_dir", starlark.String(cacheDir))
	}

	// Validate and set fallback_registries
	if fallbackVal != nil && fallbackVal != starlark.None {
		fallbackList, ok := fallbackVal.(*starlark.List)
		if !ok {
			return nil, fmt.Errorf("config.defaults: fallback_registries must be a list, got %s", fallbackVal.Type())
		}
		fallbacks := make([]*RegistryValue, fallbackList.Len())
		for i := 0; i < fallbackList.Len(); i++ {
			r, ok := fallbackList.Index(i).(*RegistryValue)
			if !ok {
				return nil, fmt.Errorf("config.defaults: fallback_registries[%d] must be a registry value, got %s", i, fallbackList.Index(i).Type())
			}
			fallbacks[i] = r
		}
		d.FallbackRegistries = fallbacks
		d.Set("fallback_registries", fallbackVal)
	}

	// Store in thread-local storage for retrieval after execution
	setConfigDefaults(thread, d)

	return d, nil
}

// ConfigModule returns the config Starlark module.
func ConfigModule() starlark.StringDict {
	return starlark.StringDict{
		"defaults": starlark.NewBuiltin("config.defaults", configDefaults),
	}
}
