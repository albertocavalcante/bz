package publisher

import (
	"fmt"

	"github.com/albertocavalcante/bz/internal/config/modules"
)

// New creates a publisher from a config.RegistryValue.
func New(reg *modules.RegistryValue) (Publisher, error) {
	if reg == nil {
		return nil, fmt.Errorf("registry configuration is required")
	}

	switch reg.Kind {
	case modules.RegistryTypeFile:
		return NewFilePublisher(reg.URL)

	case modules.RegistryTypeGit:
		branch := reg.Branch
		if branch == "" {
			branch = "main"
		}
		return NewGitPublisher(reg.URL, branch)

	case modules.RegistryTypeHTTPPut:
		var auth *Auth
		if reg.Auth != nil {
			auth = convertAuth(reg.Auth)
		}
		return NewHTTPPublisher(reg.URL, auth)

	case modules.RegistryTypeHTTP:
		// HTTP registries are read-only, cannot publish to them
		return nil, fmt.Errorf("cannot publish to HTTP registry (read-only): %s", reg.URL)

	default:
		return nil, fmt.Errorf("unsupported registry type for publishing: %s", reg.Kind)
	}
}

// convertAuth converts a Starlark AuthValue to a publisher Auth.
func convertAuth(authVal *modules.AuthValue) *Auth {
	if authVal == nil {
		return nil
	}

	return &Auth{
		Type:        authVal.Kind,
		Username:    authVal.Username,
		Password:    authVal.Password,
		TokenValue:  authVal.TokenValue,
		EnvVar:      authVal.EnvVar,
		HeaderName:  authVal.HeaderName,
		HeaderValue: authVal.HeaderValue,
	}
}
