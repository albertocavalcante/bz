package bzconfig

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// fileSchema is the on-disk TOML shape. Pointer fields record presence so
// that an absent key is distinguishable from a key set to its default.
type fileSchema struct {
	Network struct {
		Mode               *string   `toml:"mode"`
		Registry           *string   `toml:"registry"`
		FallbackRegistries *[]string `toml:"fallback_registries"`
		Timeout            *string   `toml:"timeout"`
	} `toml:"network"`
	Cache struct {
		Dir *string `toml:"dir"`
		TTL *string `toml:"ttl"`
	} `toml:"cache"`
	Commands struct {
		Disabled *[]string `toml:"disabled"`
	} `toml:"commands"`
}

// ParseFile decodes one configuration file into a layer of the given kind.
//
// Unknown keys are returned as warnings (a mistyped security setting must
// not be silently ignored). A relative [cache] dir is resolved against the
// file's directory, after "~" expansion against home.
func ParseFile(kind LayerKind, path string, data []byte, home string) (Layer, []string, error) {
	var s fileSchema
	md, err := toml.Decode(string(data), &s)
	if err != nil {
		return Layer{}, nil, fmt.Errorf("parsing %s %s: %w", kind, path, err)
	}

	// Report each unknown table or key once: skip keys below an unknown
	// table ("comands.disabled" when "comands" is reported).
	var warnings []string
	unknown := map[string]bool{}
	for _, k := range md.Undecoded() {
		unknown[k.String()] = true
	}
	for _, k := range md.Undecoded() {
		if len(k) > 1 && unknown[k[:len(k)-1].String()] {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("%s %s: unknown key %q ignored", kind, path, k.String()))
	}

	l := Layer{Kind: kind, Origin: path}
	if s.Network.Mode != nil {
		m := NetworkMode(*s.Network.Mode)
		l.Settings.Mode = &m
	}
	l.Settings.Registry = s.Network.Registry
	l.Settings.FallbackRegistries = s.Network.FallbackRegistries
	l.Settings.Timeout = s.Network.Timeout
	if s.Cache.Dir != nil {
		dir := ExpandHome(*s.Cache.Dir, home)
		if dir != "" && !filepath.IsAbs(dir) {
			dir = filepath.Join(filepath.Dir(path), dir)
		}
		l.Settings.CacheDir = &dir
	}
	l.Settings.CacheTTL = s.Cache.TTL
	l.Settings.Disabled = s.Commands.Disabled
	return l, warnings, nil
}

// ExpandHome replaces a leading "~" (alone or followed by a path separator)
// with home. "~user" forms are left unchanged.
func ExpandHome(path, home string) string {
	if home == "" || !strings.HasPrefix(path, "~") {
		return path
	}
	if path == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, rest)
	}
	if rest, ok := strings.CutPrefix(path, `~\`); ok {
		return filepath.Join(home, rest)
	}
	return path
}
