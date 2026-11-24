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
	"net/url"
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

// Configuration layers, lowest precedence first.
const (
	LayerDefault LayerKind = iota
	LayerSystem
	LayerUser
	LayerProject
	LayerEnv
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
	return s.Layer.String() + " " + DisplayPath(s.Origin)
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
	return eff, joinProblems(errs)
}

// Problems is several configuration errors reported together, so the CLI
// can list them; its text is that of errors.Join.
type Problems []error

func (p Problems) Error() string   { return errors.Join(p...).Error() }
func (p Problems) Unwrap() []error { return p }

// joinProblems is errors.Join returning Problems: nil when errs has no
// non-nil error.
func joinProblems(errs []error) error {
	var p Problems
	for _, err := range errs {
		if err != nil {
			p = append(p, err)
		}
	}
	if p == nil {
		return nil
	}
	return p
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
	if s.Registry != nil {
		if reg, err := checkRegistryURL(*s.Registry); err != nil {
			errs = append(errs, fmt.Errorf("invalid registry in %s: %w", l.source(KeyRegistry), err))
		} else {
			e.Network.Registry = reg
			e.sources[KeyRegistry] = l.source(KeyRegistry)
		}
	}
	if s.FallbackRegistries != nil {
		var bad []error
		regs := make([]string, 0, len(*s.FallbackRegistries))
		for _, r := range *s.FallbackRegistries {
			reg, err := checkRegistryURL(r)
			if err != nil {
				bad = append(bad, fmt.Errorf("invalid fallback_registries entry in %s: %w", l.source(KeyFallbackRegistries), err))
			}
			regs = append(regs, reg)
		}
		if len(bad) > 0 {
			errs = append(errs, bad...)
		} else {
			e.Network.FallbackRegistries = regs
			e.sources[KeyFallbackRegistries] = l.source(KeyFallbackRegistries)
		}
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
		if d, err := time.ParseDuration(*s.CacheTTL); *s.CacheTTL != "" && (err != nil || d < 0) {
			errs = append(errs, fmt.Errorf("invalid cache ttl %q in %s (want a Go duration such as \"24h\", or \"\" for no expiry)",
				*s.CacheTTL, l.source(KeyCacheTTL)))
		} else {
			e.Cache.TTL = *s.CacheTTL
			e.sources[KeyCacheTTL] = l.source(KeyCacheTTL)
		}
	}
	if s.Disabled != nil {
		e.Commands.Disabled = slices.Clone(*s.Disabled)
		e.sources[KeyDisabled] = l.source(KeyDisabled)
	}
	return errs
}

// RedactURL masks credentials for messages. A URL that parses with a host
// keeps everything but its userinfo. Any other value containing "@" prints
// as its scheme plus "<redacted>", because a "?", "#" or "/" there may belong
// to a password; messages always name the key and its source, so the value
// can still be found.
func RedactURL(raw string) string {
	v := strings.TrimSpace(raw)
	if !strings.Contains(v, "@") {
		return raw
	}
	if u, err := url.Parse(v); err == nil && u.Host != "" && u.Opaque == "" {
		if u.User == nil {
			return raw // the "@" is in the path, query or fragment
		}
		u.User = url.User("xxxxx")
		return u.String()
	}
	if i := strings.Index(v, "://"); i >= 0 {
		return v[:i+3] + "<redacted>"
	}
	return "<redacted>"
}

// registryHelp follows every malformed-URL message.
const registryHelp = "help: want https://host[:port]/path, file:///abs/path or an absolute path"

// urlProblem names a url.Parse failure without its message, which quotes
// the value. It uses typed errors where Go provides them; the port case is
// matched on the parser's inner cause, never on the full message, and falls
// back to "not a valid URL" if that wording changes.
func urlProblem(err error) string {
	var esc url.EscapeError
	var host url.InvalidHostError
	switch {
	case errors.As(err, &esc):
		return "bad percent-escape"
	case errors.As(err, &host):
		return "invalid character in host"
	}
	var uerr *url.Error
	if errors.As(err, &uerr) && uerr.Err != nil && strings.HasPrefix(uerr.Err.Error(), "invalid port") {
		return "invalid port"
	}
	return "not a valid URL"
}

// ValidateRegistry checks a registry value taken from source (for example
// "flag --registry") with the rules that apply to configured registries. The
// error names the source and shows the value without credentials.
func ValidateRegistry(source, raw string) error {
	if _, err := checkRegistryURL(raw); err != nil {
		return fmt.Errorf("invalid registry in %s: %w", source, err)
	}
	return nil
}

// checkRegistryURL validates a registry value and returns it with
// surrounding whitespace removed, which is the form callers store (the
// registry client matches URL prefixes on the raw string). It accepts the
// forms bz supports: http:// or https:// URLs with a host name, file://
// URLs with a path, and absolute filesystem paths (Unix or Windows drive
// paths). A value with an interior control or non-printable character is
// rejected.
func checkRegistryURL(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	switch {
	case v == "":
		return "", errors.New("empty registry URL")
	case !printable(v):
		return "", fmt.Errorf("%q: contains control or non-printable characters", RedactURL(raw))
	case strings.HasPrefix(v, "https://"), strings.HasPrefix(v, "http://"):
		u, err := url.Parse(v)
		if err != nil {
			return "", fmt.Errorf("%q: %s\n%s", RedactURL(raw), urlProblem(err), registryHelp)
		}
		if u.Hostname() == "" {
			return "", fmt.Errorf("%q: missing host\n%s", RedactURL(raw), registryHelp)
		}
		return v, nil
	case strings.HasPrefix(v, "file://"):
		if strings.TrimPrefix(v, "file://") == "" {
			return "", fmt.Errorf("%q: missing path", RedactURL(raw))
		}
		return v, nil
	case strings.HasPrefix(v, "/"), isWindowsDrivePath(v):
		return v, nil
	default:
		return "", fmt.Errorf("%q: want an https://, http:// or file:// URL or an absolute path", RedactURL(raw))
	}
}

func isWindowsDrivePath(p string) bool {
	if len(p) < 3 || (p[2] != '\\' && p[2] != '/') || p[1] != ':' {
		return false
	}
	c := p[0] | 0x20
	return c >= 'a' && c <= 'z'
}
