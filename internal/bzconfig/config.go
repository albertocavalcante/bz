// Package bzconfig computes bz's effective configuration.
//
// Configuration comes from layers, lowest precedence first:
//
//	built-in defaults < system file < user file < project file
//	< environment variables < command-line flags
//
// Each layer records only the settings it sets explicitly (see [Settings]),
// so a higher layer can set a value back to its default and a lower layer's
// value survives when a higher layer does not mention it. [Merge] combines
// the layers once, validates every value and records which layer set each
// effective value (see [Effective.Source]).
//
// The package is free of process-global state: filesystem and environment
// access goes through an injected [Environment].
package bzconfig

import (
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

// Keys returns every setting key in documentation order.
func Keys() []Key {
	return []Key{KeyMode, KeyRegistry, KeyFallbackRegistries, KeyTimeout, KeyCacheDir, KeyCacheTTL, KeyDisabled}
}

// LayerKind identifies a configuration layer. Higher values take precedence.
type LayerKind int

// Configuration layers, lowest precedence first.
const (
	LayerDefault LayerKind = iota
	LayerSystem
	LayerUser
	LayerProject
	LayerEnv
	LayerFlag
)

func (k LayerKind) String() string {
	switch k {
	case LayerDefault:
		return "default"
	case LayerSystem:
		return "system config"
	case LayerUser:
		return "user config"
	case LayerProject:
		return "project config"
	case LayerEnv:
		return "environment"
	case LayerFlag:
		return "flag"
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

// String renders the source for messages, e.g. "user config
// /home/me/.config/bz/config.toml" or "environment BZ_OFFLINE".
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
	// Files lists the configuration files that were read, lowest
	// precedence first.
	Files []string
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
// lowest precedence first) and validates every explicitly set value. An
// invalid value is an error naming its source even when a higher layer
// overrides it, because it is still a mistake in that file or variable.
func Merge(defaults Config, layers ...Layer) (*Effective, error) {
	eff := &Effective{Config: defaults, sources: map[Key]Source{}}
	for _, k := range Keys() {
		eff.sources[k] = Source{Layer: LayerDefault}
	}
	eff.Network.FallbackRegistries = slices.Clone(defaults.Network.FallbackRegistries)
	eff.Commands.Disabled = slices.Clone(defaults.Commands.Disabled)

	for _, l := range layers {
		if err := eff.apply(l); err != nil {
			return nil, err
		}
	}
	return eff, nil
}

func (e *Effective) apply(l Layer) error {
	s := l.Settings
	if s.Mode != nil {
		if !s.Mode.Valid() {
			return fmt.Errorf("invalid network mode %q in %s (want %q, %q or %q)",
				*s.Mode, l.source(KeyMode), NetworkModeOnline, NetworkModePreferOffline, NetworkModeOffline)
		}
		e.Network.Mode = *s.Mode
		e.sources[KeyMode] = l.source(KeyMode)
	}
	if s.Registry != nil {
		if strings.TrimSpace(*s.Registry) == "" {
			return fmt.Errorf("empty registry in %s", l.source(KeyRegistry))
		}
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
			return fmt.Errorf("invalid network timeout %q in %s (want a positive Go duration such as \"30s\")",
				*s.Timeout, l.source(KeyTimeout))
		}
		e.Network.Timeout = *s.Timeout
		e.Network.TimeoutDuration = d
		e.sources[KeyTimeout] = l.source(KeyTimeout)
	}
	if s.CacheDir != nil {
		if strings.TrimSpace(*s.CacheDir) == "" {
			return fmt.Errorf("empty cache dir in %s", l.source(KeyCacheDir))
		}
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
	return nil
}

// FlagLayer builds the command-line flag layer from the global flags.
// Only flags that are set contribute; --offline and --prefer-offline are
// mutually exclusive and validated by the caller.
func FlagLayer(offline, preferOffline bool, registry string) Layer {
	l := Layer{Kind: LayerFlag, KeyOrigins: map[Key]string{}}
	switch {
	case offline:
		m := NetworkModeOffline
		l.Settings.Mode = &m
		l.KeyOrigins[KeyMode] = "--offline"
	case preferOffline:
		m := NetworkModePreferOffline
		l.Settings.Mode = &m
		l.KeyOrigins[KeyMode] = "--prefer-offline"
	}
	if registry != "" {
		l.Settings.Registry = &registry
		l.KeyOrigins[KeyRegistry] = "--registry"
	}
	return l
}
