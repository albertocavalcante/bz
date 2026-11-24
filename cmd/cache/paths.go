package cache

import (
	"github.com/albertocavalcante/bz/internal/cli"
)

// resolveCacheDir returns the cache directory for a cache command:
// the command's --cache-dir flag (override) wins, then BZ_CACHE_DIR, then
// [cache] dir from the config files, then the platform default (see
// bzconfig.DefaultCacheDir).
func resolveCacheDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return cli.CacheDir()
}
