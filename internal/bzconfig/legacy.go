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
// loader for system, user and project files is introduced separately. Like
// earlier releases it is tolerant: an invalid value (an unknown mode, a bad
// timeout, an unparsable boolean variable) is ignored and the remaining
// settings still apply. Only an unreadable or malformed file is an error.
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
		l, err := ParseFile(LayerProject, path, data, home)
		if err != nil {
			return nil, err
		}
		layers = append(layers, l)
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	envLayer, _ := EnvLayer(os.LookupEnv, home) // invalid booleans read as false, as before
	layers = append(layers, envLayer)
	eff, _ := Merge(Defaults(cacheDir), layers...) // invalid values are skipped, as before
	return eff, nil
}
