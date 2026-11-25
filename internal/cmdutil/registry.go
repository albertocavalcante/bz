package cmdutil

import (
	"fmt"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/registry"
)

// OpenRegistry checks the effective offline setting before returning a
// registry that can make network requests. Filesystem registries stay usable.
func OpenRegistry(command, url string) (registry.Registry, error) {
	reg, err := registry.New(url)
	if err != nil {
		return nil, fmt.Errorf("invalid registry: %w", err)
	}
	if err := CheckRegistryNetwork(command, reg); err != nil {
		return nil, err
	}
	return reg, nil
}

// CheckRegistryNetwork permits filesystem registry reads in offline mode.
func CheckRegistryNetwork(command string, reg registry.Registry) error {
	if reg.Type() == registry.TypeFile {
		return nil
	}
	return cli.CheckOfflineAllowed(command)
}
