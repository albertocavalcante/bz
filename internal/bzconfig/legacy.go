package bzconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// LoadProjectFile computes the effective configuration from the built-in
// defaults, the file at path (when it exists) and the environment.
//
// It keeps the single-file behavior of earlier releases while the layered
// loader for system, user and project files is introduced separately.
func LoadProjectFile(path string) (*Effective, error) {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(".cache", "bz")
	if home != "" {
		cacheDir = filepath.Join(home, ".cache", "bz")
	}

	var layers []Layer
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		l, _, err := ParseFile(LayerProject, path, data, home)
		if err != nil {
			return nil, err
		}
		layers = append(layers, l)
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	envLayer, err := EnvLayer(os.LookupEnv, home)
	if err != nil {
		return nil, err
	}
	layers = append(layers, envLayer)
	return Merge(Defaults(cacheDir), layers...)
}
