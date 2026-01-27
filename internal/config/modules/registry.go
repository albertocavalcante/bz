package modules

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/albertocavalcante/bz/internal/starlark/value"
)

// RegistryType identifies the registry backend type.
const (
	RegistryTypeHTTP    = "http"
	RegistryTypeFile    = "file"
	RegistryTypeGit     = "git"
	RegistryTypeHTTPPut = "http_put"
)

// RegistryValue represents a registry configuration in Starlark.
type RegistryValue struct {
	value.Base
	value.AttrAccessor

	// Kind is the registry type: "http", "file", "git", or "http_put".
	Kind string

	// URL is the registry URL (for http, git, http_put) or path (for file).
	URL string

	// Branch is the git branch (for git registries).
	Branch string

	// Ref is the git ref (for git registries).
	Ref string

	// Auth is the authentication configuration (for http_put registries).
	Auth *AuthValue
}

// Ensure RegistryValue implements the required interfaces.
var (
	_ starlark.Value    = (*RegistryValue)(nil)
	_ starlark.HasAttrs = (*RegistryValue)(nil)
)

// String returns a string representation of the registry value.
func (r *RegistryValue) String() string {
	switch r.Kind {
	case RegistryTypeHTTP:
		return fmt.Sprintf("registry.http(%q)", r.URL)
	case RegistryTypeFile:
		return fmt.Sprintf("registry.file(%q)", r.URL)
	case RegistryTypeGit:
		if r.Branch != "" {
			return fmt.Sprintf("registry.git(url=%q, branch=%q)", r.URL, r.Branch)
		}
		if r.Ref != "" {
			return fmt.Sprintf("registry.git(url=%q, ref=%q)", r.URL, r.Ref)
		}
		return fmt.Sprintf("registry.git(url=%q)", r.URL)
	case RegistryTypeHTTPPut:
		if r.Auth != nil {
			return fmt.Sprintf("registry.http_put(url=%q, auth=%s)", r.URL, r.Auth.String())
		}
		return fmt.Sprintf("registry.http_put(url=%q)", r.URL)
	default:
		return "registry"
	}
}

// newRegistryValue creates a new RegistryValue with common initialization.
func newRegistryValue(regType, url string) *RegistryValue {
	r := &RegistryValue{
		Base: value.NewBase("registry"),
		Kind: regType,
		URL:  url,
	}
	r.AttrAccessor = value.NewAttrAccessor()
	r.Set("type", starlark.String(regType))
	r.Set("url", starlark.String(url))
	return r
}

// registryHTTP creates a registry.http configuration.
func registryHTTP(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("registry.http", args, kwargs)

	var url string
	if err := u.Required("url", &url); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	return newRegistryValue(RegistryTypeHTTP, url), nil
}

// registryFile creates a registry.file configuration.
func registryFile(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("registry.file", args, kwargs)

	var path string
	if err := u.Required("path", &path); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	// For file registries, we store the path in the URL field
	return newRegistryValue(RegistryTypeFile, path), nil
}

// registryGit creates a registry.git configuration.
func registryGit(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("registry.git", args, kwargs)

	var url, branch, ref string
	if err := u.Required("url", &url); err != nil {
		return nil, err
	}
	if err := u.Optional("branch", &branch, ""); err != nil {
		return nil, err
	}
	if err := u.Optional("ref", &ref, ""); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	// Cannot specify both branch and ref
	if branch != "" && ref != "" {
		return nil, fmt.Errorf("registry.git: cannot specify both 'branch' and 'ref'")
	}

	r := newRegistryValue(RegistryTypeGit, url)
	r.Branch = branch
	r.Ref = ref
	if branch != "" {
		r.Set("branch", starlark.String(branch))
	}
	if ref != "" {
		r.Set("ref", starlark.String(ref))
	}
	return r, nil
}

// registryHTTPPut creates a registry.http_put configuration.
func registryHTTPPut(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("registry.http_put", args, kwargs)

	var url string
	var authVal starlark.Value
	if err := u.Required("url", &url); err != nil {
		return nil, err
	}
	if err := u.Optional("auth", &authVal, starlark.None); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	r := newRegistryValue(RegistryTypeHTTPPut, url)

	// Validate auth if provided
	if authVal != nil && authVal != starlark.None {
		auth, ok := authVal.(*AuthValue)
		if !ok {
			return nil, fmt.Errorf("registry.http_put: auth must be an auth value, got %s", authVal.Type())
		}
		r.Auth = auth
		r.Set("auth", auth)
	}

	return r, nil
}

// RegistryModule returns the registry Starlark module.
func RegistryModule() starlark.StringDict {
	return starlark.StringDict{
		"http":     starlark.NewBuiltin("registry.http", registryHTTP),
		"file":     starlark.NewBuiltin("registry.file", registryFile),
		"git":      starlark.NewBuiltin("registry.git", registryGit),
		"http_put": starlark.NewBuiltin("registry.http_put", registryHTTPPut),
	}
}
