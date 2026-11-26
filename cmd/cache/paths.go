package cache

import "github.com/albertocavalcante/bz/internal/cachepath"

func resolveCacheDir(override string) (string, error) {
	return cachepath.Resolve(override)
}
