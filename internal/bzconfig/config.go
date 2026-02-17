// Package bzconfig provides configuration loading and management for bz.
// It supports TOML configuration files with a layered precedence system.
package bzconfig

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"
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

// Default values for configuration.
const (
	DefaultRegistry = "https://bcr.bazel.build"
	DefaultTimeout  = "30s"
	DefaultTTL      = ""
)

// Config represents the bz configuration.
type Config struct {
	Network  NetworkConfig  `toml:"network"`
	Cache    CacheConfig    `toml:"cache"`
	Commands CommandsConfig `toml:"commands"`
}

// NetworkConfig contains network-related settings.
type NetworkConfig struct {
	Mode               NetworkMode `toml:"mode"`
	Registry           string      `toml:"registry"`
	FallbackRegistries []string    `toml:"fallback_registries"`
	Timeout            string      `toml:"timeout"`
}

// CacheConfig contains cache-related settings.
type CacheConfig struct {
	Dir string `toml:"dir"`
	TTL string `toml:"ttl"`
}

// CommandsConfig contains command-related settings.
type CommandsConfig struct {
	Disabled []string `toml:"disabled"`
}

// LoadOption configures the Load function.
type LoadOption func(*loadOptions)

type loadOptions struct {
	systemConfig  string
	userConfig    string
	projectConfig string
}

// WithSystemConfig sets the system config path (e.g., /etc/bz/config.toml).
func WithSystemConfig(path string) LoadOption {
	return func(opts *loadOptions) {
		opts.systemConfig = path
	}
}

// WithUserConfig sets the user config path (e.g., ~/.config/bz/config.toml).
func WithUserConfig(path string) LoadOption {
	return func(opts *loadOptions) {
		opts.userConfig = path
	}
}

// WithProjectConfig sets the project config path (e.g., .bzconfig.toml).
func WithProjectConfig(path string) LoadOption {
	return func(opts *loadOptions) {
		opts.projectConfig = path
	}
}

// Load loads configuration with precedence:
// 1. CLI flags (passed as overrides)
// 2. Environment variables (BZ_OFFLINE, BZ_REGISTRY, etc.)
// 3. Project config (.bzconfig.toml)
// 4. User config (~/.config/bz/config.toml)
// 5. System config (/etc/bz/config.toml)
// 6. Built-in defaults
func Load(overrides *Config, opts ...LoadOption) (*Config, error) {
	options := &loadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Start with defaults
	cfg := &Config{}
	cfg.applyDefaults()

	// Layer 6 -> 5: Load system config
	if options.systemConfig != "" {
		if _, err := os.Stat(options.systemConfig); err == nil {
			systemCfg, err := LoadFromFile(options.systemConfig)
			if err != nil {
				return nil, err
			}
			cfg.merge(systemCfg)
		}
	}

	// Layer 5 -> 4: Load user config
	if options.userConfig != "" {
		if _, err := os.Stat(options.userConfig); err == nil {
			userCfg, err := LoadFromFile(options.userConfig)
			if err != nil {
				return nil, err
			}
			cfg.merge(userCfg)
		}
	}

	// Layer 4 -> 3: Load project config
	if options.projectConfig != "" {
		if _, err := os.Stat(options.projectConfig); err == nil {
			projectCfg, err := LoadFromFile(options.projectConfig)
			if err != nil {
				return nil, err
			}
			cfg.merge(projectCfg)
		}
	}

	// Layer 3 -> 2: Apply environment variables
	cfg.applyEnvVars()

	// Layer 2 -> 1: Apply CLI overrides
	if overrides != nil {
		cfg.mergeOverrides(overrides)
	}

	return cfg, nil
}

// LoadFromFile loads configuration from a TOML file.
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for missing values
	cfg.applyDefaults()

	return cfg, nil
}

// IsOffline returns true if network mode is offline.
func (c *Config) IsOffline() bool {
	return c.Network.Mode == NetworkModeOffline
}

// IsPreferOffline returns true if network mode is prefer-offline.
func (c *Config) IsPreferOffline() bool {
	return c.Network.Mode == NetworkModePreferOffline
}

// IsCommandDisabled returns true if the command is disabled.
func (c *Config) IsCommandDisabled(cmd string) bool {
	return slices.Contains(c.Commands.Disabled, cmd)
}

// GetRegistry returns the effective registry URL.
func (c *Config) GetRegistry() string {
	if c.Network.Registry == "" {
		return DefaultRegistry
	}
	return c.Network.Registry
}

// applyDefaults applies default values to empty fields.
func (c *Config) applyDefaults() {
	if c.Network.Mode == "" {
		c.Network.Mode = NetworkModeOnline
	}
	if c.Network.Registry == "" {
		c.Network.Registry = DefaultRegistry
	}
	if c.Network.Timeout == "" {
		c.Network.Timeout = DefaultTimeout
	}
	if c.Cache.Dir == "" {
		c.Cache.Dir = defaultCacheDir()
	}
}

// defaultCacheDir returns the default cache directory.
func defaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cache/bz"
	}
	return filepath.Join(home, ".cache", "bz")
}

// merge merges values from another config, overwriting non-empty values.
func (c *Config) merge(other *Config) {
	if other == nil {
		return
	}

	// Network
	if other.Network.Mode != "" {
		c.Network.Mode = other.Network.Mode
	}
	if other.Network.Registry != "" && other.Network.Registry != DefaultRegistry {
		c.Network.Registry = other.Network.Registry
	}
	if len(other.Network.FallbackRegistries) > 0 {
		c.Network.FallbackRegistries = other.Network.FallbackRegistries
	}
	if other.Network.Timeout != "" && other.Network.Timeout != DefaultTimeout {
		c.Network.Timeout = other.Network.Timeout
	}

	// Cache
	if other.Cache.Dir != "" && other.Cache.Dir != defaultCacheDir() {
		c.Cache.Dir = other.Cache.Dir
	}
	if other.Cache.TTL != "" {
		c.Cache.TTL = other.Cache.TTL
	}

	// Commands
	if len(other.Commands.Disabled) > 0 {
		c.Commands.Disabled = other.Commands.Disabled
	}
}

// mergeOverrides merges CLI overrides, which always take precedence.
func (c *Config) mergeOverrides(overrides *Config) {
	if overrides == nil {
		return
	}

	// Network - overrides always win if set
	if overrides.Network.Mode != "" {
		c.Network.Mode = overrides.Network.Mode
	}
	if overrides.Network.Registry != "" {
		c.Network.Registry = overrides.Network.Registry
	}
	if len(overrides.Network.FallbackRegistries) > 0 {
		c.Network.FallbackRegistries = overrides.Network.FallbackRegistries
	}
	if overrides.Network.Timeout != "" {
		c.Network.Timeout = overrides.Network.Timeout
	}

	// Cache
	if overrides.Cache.Dir != "" {
		c.Cache.Dir = overrides.Cache.Dir
	}
	if overrides.Cache.TTL != "" {
		c.Cache.TTL = overrides.Cache.TTL
	}

	// Commands
	if len(overrides.Commands.Disabled) > 0 {
		c.Commands.Disabled = overrides.Commands.Disabled
	}
}
