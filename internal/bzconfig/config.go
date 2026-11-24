// Package bzconfig computes bz's effective configuration.
//
// Configuration comes from layers applied over built-in defaults, lowest
// precedence first. Each layer records only the settings it sets explicitly
// (see [Settings]), so a higher layer can set a value back to its default
// and a lower layer's value survives when a higher layer does not mention
// it. [Merge] combines the layers once, applies the defaults once and
// records which layer set each effective value (see [Effective.Source]).
package bzconfig

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// NetworkMode defines how bz handles network access.
type NetworkMode string

const (
	// NetworkModeOnline is the default mode: network-first.
	NetworkModeOnline NetworkMode = "online"
	// NetworkModePreferOffline is cache-first, fallback to network.
	NetworkModePreferOffline NetworkMode = "prefer-offline"
	// NetworkModeOffline is cache-only, fail if not cached.
	NetworkModeOffline NetworkMode = "offline"
)

// Valid reports whether m is one of the known network modes.
func (m NetworkMode) Valid() bool {
	switch m {
	case NetworkModeOnline, NetworkModePreferOffline, NetworkModeOffline:
		return true
	default:
		return false
	}
}

// Default values for configuration.
const (
	DefaultRegistry = "https://bcr.bazel.build"
	DefaultTimeout  = "30s"
	DefaultTTL      = ""
)

// Config is a fully resolved configuration: every field holds its effective
// value, defaults included.
type Config struct {
	Network  NetworkConfig
	Cache    CacheConfig
	Commands CommandsConfig
}

// NetworkConfig contains network-related settings.
type NetworkConfig struct {
	Mode               NetworkMode
	Registry           string
	FallbackRegistries []string
	// Timeout is the request timeout as written in configuration
	// (Go duration syntax); TimeoutDuration is its parsed value.
	Timeout         string
	TimeoutDuration time.Duration
}

// CacheConfig contains cache-related settings.
type CacheConfig struct {
	Dir string
	TTL string
}

// CommandsConfig contains command-related settings.
type CommandsConfig struct {
	Disabled []string
}

// IsOffline returns true if network mode is offline.
func (c *Config) IsOffline() bool {
	return c.Network.Mode == NetworkModeOffline
}

// IsPreferOffline returns true if network mode is prefer-offline.
func (c *Config) IsPreferOffline() bool {
	return c.Network.Mode == NetworkModePreferOffline
}

// Key names one setting, spelled as its dotted TOML key.
type Key string

// Setting keys.
const (
	KeyMode               Key = "network.mode"
	KeyRegistry           Key = "network.registry"
	KeyFallbackRegistries Key = "network.fallback_registries"
	KeyTimeout            Key = "network.timeout"
	KeyCacheDir           Key = "cache.dir"
	KeyCacheTTL           Key = "cache.ttl"
	KeyDisabled           Key = "commands.disabled"
)

// LayerKind identifies a configuration layer. Higher values take precedence.
type LayerKind int

// Configuration layers, lowest precedence first. The gaps leave room for
// layers that are not loaded yet.
const (
	LayerDefault LayerKind = 0
	LayerProject LayerKind = 3
	LayerEnv     LayerKind = 4
)

func (k LayerKind) String() string {
	switch k {
	case LayerDefault:
		return "default"
	case LayerProject:
		return "project config"
	case LayerEnv:
		return "environment"
	default:
		return fmt.Sprintf("layer(%d)", int(k))
	}
}

// Source says where an effective value came from.
type Source struct {
	Layer LayerKind
	// Origin is the file path, environment variable or flag that set the
	// value; empty for built-in defaults.
	Origin string
}

// String renders the source for messages, e.g. "project config
// /work/.bzconfig.toml" or "environment BZ_OFFLINE".
func (s Source) String() string {
	if s.Origin == "" {
		return s.Layer.String()
	}
	return s.Layer.String() + " " + s.Origin
}

// Settings holds the values one layer sets explicitly. A nil field means the
// layer does not mention that setting.
type Settings struct {
	Mode               *NetworkMode
	Registry           *string
	FallbackRegistries *[]string
	Timeout            *string
	CacheDir           *string
	CacheTTL           *string
	Disabled           *[]string
}

// Layer is one configuration source.
type Layer struct {
	Kind LayerKind
	// Origin applies to every key unless KeyOrigins names a more specific
	// origin (used by the environment and flag layers, where each key has
	// its own variable or flag name).
	Origin     string
	KeyOrigins map[Key]string
	Settings   Settings
}

func (l Layer) source(k Key) Source {
	if o, ok := l.KeyOrigins[k]; ok {
		return Source{Layer: l.Kind, Origin: o}
	}
	return Source{Layer: l.Kind, Origin: l.Origin}
}

// Effective is the merged configuration plus the source of each value.
type Effective struct {
	Config
	sources map[Key]Source
}

// Source returns the layer that set key's effective value.
func (e *Effective) Source(key Key) Source {
	if s, ok := e.sources[key]; ok {
		return s
	}
	return Source{Layer: LayerDefault}
}

// Defaults returns the built-in defaults. cacheDir is the platform default
// cache directory (see [DefaultCacheDir]).
func Defaults(cacheDir string) Config {
	timeout, _ := time.ParseDuration(DefaultTimeout)
	return Config{
		Network: NetworkConfig{
			Mode:            NetworkModeOnline,
			Registry:        DefaultRegistry,
			Timeout:         DefaultTimeout,
			TimeoutDuration: timeout,
		},
		Cache: CacheConfig{Dir: cacheDir, TTL: DefaultTTL},
	}
}

// Merge applies layers over defaults in the order given (callers pass them
// lowest precedence first). Each explicitly set value is validated; an
// invalid value is skipped, so the key keeps the value from the layers
// below, and reported in the returned error together with its source.
// The returned configuration is always usable: strict callers treat the
// error as fatal, tolerant callers may ignore it.
func Merge(defaults Config, layers ...Layer) (*Effective, error) {
	eff := &Effective{Config: defaults, sources: map[Key]Source{}}
	eff.Network.FallbackRegistries = slices.Clone(defaults.Network.FallbackRegistries)
	eff.Commands.Disabled = slices.Clone(defaults.Commands.Disabled)

	errs := make([]error, 0, len(layers))
	for _, l := range layers {
		errs = append(errs, eff.apply(l)...)
	}
	return eff, errors.Join(errs...)
}

func (e *Effective) apply(l Layer) []error {
	var errs []error
	s := l.Settings
	if s.Mode != nil && !s.Mode.Valid() {
		errs = append(errs, fmt.Errorf("invalid network mode %q in %s (want %q, %q or %q)",
			*s.Mode, l.source(KeyMode), NetworkModeOnline, NetworkModePreferOffline, NetworkModeOffline))
	} else if s.Mode != nil {
		e.Network.Mode = *s.Mode
		e.sources[KeyMode] = l.source(KeyMode)
	}
	if s.Registry != nil && strings.TrimSpace(*s.Registry) == "" {
		errs = append(errs, fmt.Errorf("empty registry in %s", l.source(KeyRegistry)))
	} else if s.Registry != nil {
		e.Network.Registry = *s.Registry
		e.sources[KeyRegistry] = l.source(KeyRegistry)
	}
	if s.FallbackRegistries != nil {
		e.Network.FallbackRegistries = slices.Clone(*s.FallbackRegistries)
		e.sources[KeyFallbackRegistries] = l.source(KeyFallbackRegistries)
	}
	if s.Timeout != nil {
		d, err := time.ParseDuration(*s.Timeout)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("invalid network timeout %q in %s (want a positive Go duration such as \"30s\")",
				*s.Timeout, l.source(KeyTimeout)))
		} else {
			e.Network.Timeout = *s.Timeout
			e.Network.TimeoutDuration = d
			e.sources[KeyTimeout] = l.source(KeyTimeout)
		}
	}
	if s.CacheDir != nil && strings.TrimSpace(*s.CacheDir) == "" {
		errs = append(errs, fmt.Errorf("empty cache dir in %s", l.source(KeyCacheDir)))
	} else if s.CacheDir != nil {
		e.Cache.Dir = *s.CacheDir
		e.sources[KeyCacheDir] = l.source(KeyCacheDir)
	}
	if s.CacheTTL != nil {
		e.Cache.TTL = *s.CacheTTL
		e.sources[KeyCacheTTL] = l.source(KeyCacheTTL)
	}
	if s.Disabled != nil {
		e.Commands.Disabled = slices.Clone(*s.Disabled)
		e.sources[KeyDisabled] = l.source(KeyDisabled)
	}
	return errs
}
