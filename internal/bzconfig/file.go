package bzconfig

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// fileSchema is the on-disk TOML shape. Pointer fields record presence so
// that an absent key is distinguishable from a key set to its default.
// The want tag is the fixed text shown when a value has the wrong type.
type fileSchema struct {
	Network struct {
		Mode               *string   `toml:"mode" want:"a string such as \"online\""`
		Registry           *string   `toml:"registry" want:"a string such as \"https://bcr.bazel.build\""`
		FallbackRegistries *[]string `toml:"fallback_registries" want:"an array of strings such as [\"https://bcr.bazel.build\"]"`
		Timeout            *string   `toml:"timeout" want:"a string such as \"30s\""`
	} `toml:"network" want:"a table such as [network]"`
	Cache struct {
		Dir *string `toml:"dir" want:"a string such as \"~/.cache/bz\""`
		TTL *string `toml:"ttl" want:"a string such as \"24h\""`
	} `toml:"cache" want:"a table such as [cache]"`
	Commands struct {
		Disabled *[]string `toml:"disabled" want:"an array of strings such as [\"audit\"]"`
	} `toml:"commands" want:"a table such as [commands]"`
}

// ParseFile decodes one configuration file into a layer of the given kind.
//
// A leading "~" in [cache] dir is expanded against home. Entries of
// [commands] disabled are trimmed and empty entries dropped, as for
// BZ_DISABLE_COMMANDS.
func ParseFile(kind LayerKind, path string, data []byte, home string) (Layer, error) {
	var s fileSchema
	if _, err := toml.Decode(string(data), &s); err != nil {
		return Layer{}, fmt.Errorf("parsing %s %s: %s", kind, DisplayPath(path), describeTOMLError(err, data))
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
