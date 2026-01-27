package publisher

import (
	"fmt"
	"os"

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

	auth := &Auth{}

	switch authVal.Kind {
	case modules.AuthTypeBasic:
		auth.Type = AuthTypeBasic
		auth.Username = authVal.Username
		auth.Password = authVal.Password

	case modules.AuthTypeBearerToken:
		auth.Type = AuthTypeBearer
		if authVal.EnvVar != "" {
			auth.EnvVar = authVal.EnvVar
			auth.Token = os.Getenv(authVal.EnvVar)
		} else {
			auth.Token = authVal.TokenValue
		}

	case modules.AuthTypeHeader:
		auth.Type = AuthTypeHeader
		auth.HeaderName = authVal.HeaderName
		auth.HeaderValue = authVal.HeaderValue
	}

	return auth
}
