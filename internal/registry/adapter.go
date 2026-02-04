package registry

import (
	"context"
	"errors"

	"github.com/albertocavalcante/go-bcr"
)

// bcrRegistry is an internal interface that represents a go-bcr Registry
// with additional Type() and String() methods.
type bcrRegistry interface {
	bcr.Registry
	Type() string
	String() string
}

// bcrModuleLister is an internal interface for registries that support listing.
type bcrModuleLister interface {
	ListModules(ctx context.Context) ([]string, error)
}

// bcrAdapter wraps a go-bcr Registry to implement bz's Registry interface.
type bcrAdapter struct {
	reg bcrRegistry
}

// FromBCR wraps a go-bcr Registry to implement bz's Registry interface.
// The underlying registry must implement Type() and String() methods.
func FromBCR(reg bcrRegistry) Registry {
	return &bcrAdapter{reg: reg}
}

// GetMetadata fetches the metadata.json for a module.
func (a *bcrAdapter) GetMetadata(ctx context.Context, module string) (*Metadata, error) {
	meta, err := a.reg.Metadata(ctx, module)
	if err != nil {
		return nil, convertError(err)
	}
	return convertMetadata(meta), nil
}

// GetModuleBazel fetches the MODULE.bazel content for a specific version.
func (a *bcrAdapter) GetModuleBazel(ctx context.Context, module, version string) ([]byte, error) {
	data, err := a.reg.ModuleFile(ctx, module, version)
	if err != nil {
		return nil, convertError(err)
	}
	return data, nil
}

// ListModules returns all available module names.
func (a *bcrAdapter) ListModules(ctx context.Context) ([]string, error) {
	lister, ok := a.reg.(bcrModuleLister)
	if !ok {
		return nil, ErrListingNotSupported
	}
	modules, err := lister.ListModules(ctx)
	if err != nil {
		return nil, convertError(err)
	}
	return modules, nil
}

// Type returns the registry type identifier.
func (a *bcrAdapter) Type() string {
	return a.reg.Type()
}

// String returns a human-readable representation.
func (a *bcrAdapter) String() string {
	return a.reg.String()
}

// convertError converts bcr errors to bz registry errors.
func convertError(err error) error {
	if err == nil {
		return nil
	}

	// Check for NotFoundError
	var nf *bcr.NotFoundError
	if errors.As(err, &nf) {
		if nf.Version != "" {
			return ErrVersionNotFound
		}
		return ErrModuleNotFound
	}

	// Check for ErrNotFound sentinel
	if errors.Is(err, bcr.ErrNotFound) {
		return ErrModuleNotFound
	}

	// Check for ErrListingNotSupported
	if errors.Is(err, bcr.ErrListingNotSupported) {
		return ErrListingNotSupported
	}

	// Return original error
	return err
}

// convertMetadata converts bcr.Metadata to registry.Metadata.
func convertMetadata(meta *bcr.Metadata) *Metadata {
	if meta == nil {
		return nil
	}

	result := &Metadata{
		Homepage:       meta.Homepage,
		Repository:     meta.Repository,
		Versions:       meta.Versions,
		YankedVersions: meta.YankedVersions,
	}

	// Convert maintainers
	if len(meta.Maintainers) > 0 {
		result.Maintainers = make([]Maintainer, len(meta.Maintainers))
		for i, m := range meta.Maintainers {
			result.Maintainers[i] = Maintainer{
				Name:         m.Name,
				Email:        m.Email,
				GitHub:       m.GitHub,
				GitHubUserID: int(m.GitHubID),
			}
		}
	}

	return result
}

// Verify bcrAdapter implements Registry.
var _ Registry = (*bcrAdapter)(nil)
