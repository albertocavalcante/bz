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
// A leading "~" in [cache] dir is expanded against home. Entries of
// [commands] disabled are trimmed and empty entries dropped, as for
// BZ_DISABLE_COMMANDS.
func ParseFile(kind LayerKind, path string, data []byte, home string) (Layer, error) {
	var s fileSchema
	if _, err := toml.Decode(string(data), &s); err != nil {
		return Layer{}, fmt.Errorf("parsing %s %s: %w", kind, path, err)
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
		l.Settings.CacheDir = &dir
	}
	l.Settings.CacheTTL = s.Cache.TTL
	if s.Commands.Disabled != nil {
		list := make([]string, 0, len(*s.Commands.Disabled))
		for _, d := range *s.Commands.Disabled {
			if d = strings.TrimSpace(d); d != "" {
				list = append(list, d)
			}
		}
		l.Settings.Disabled = &list
	}
	return l, nil
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
