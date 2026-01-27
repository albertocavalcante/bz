package modules

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/albertocavalcante/bz/internal/starlark/value"
)

// AuthType identifies the authentication method.
const (
	AuthTypeBasic       = "basic"
	AuthTypeBearerToken = "bearer_token"
	AuthTypeHeader      = "header"
)

// AuthValue represents an authentication configuration in Starlark.
type AuthValue struct {
	value.Base
	value.AttrAccessor

	// Kind is the authentication method: "basic", "bearer_token", or "header".
	Kind string

	// Username for basic auth.
	Username string

	// Password for basic auth.
	Password string

	// EnvVar is the environment variable name for bearer token.
	EnvVar string

	// TokenValue is a static bearer token value.
	TokenValue string

	// HeaderName is the custom header name for header auth.
	HeaderName string

	// HeaderValue is the custom header value for header auth.
	HeaderValue string
}

// Ensure AuthValue implements the required interfaces.
var (
	_ starlark.Value    = (*AuthValue)(nil)
	_ starlark.HasAttrs = (*AuthValue)(nil)
)

// String returns a string representation of the auth value.
func (a *AuthValue) String() string {
	switch a.Kind {
	case AuthTypeBasic:
		return fmt.Sprintf("auth.basic(username=%q)", a.Username)
	case AuthTypeBearerToken:
		if a.EnvVar != "" {
			return fmt.Sprintf("auth.bearer_token(env=%q)", a.EnvVar)
		}
		return "auth.bearer_token(value=***)"
	case AuthTypeHeader:
		return fmt.Sprintf("auth.header(name=%q)", a.HeaderName)
	default:
		return "auth"
	}
}

// newAuthValue creates a new AuthValue with common initialization.
func newAuthValue(authType string) *AuthValue {
	a := &AuthValue{
		Base: value.NewBase("auth"),
		Kind: authType,
	}
	a.AttrAccessor = value.NewAttrAccessor()
	a.Set("type", starlark.String(authType))
	return a
}

// authBasic creates an auth.basic configuration.
func authBasic(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("auth.basic", args, kwargs)

	var username, password string
	if err := u.Required("username", &username); err != nil {
		return nil, err
	}
	if err := u.Required("password", &password); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	a := newAuthValue(AuthTypeBasic)
	a.Username = username
	a.Password = password
	a.Set("username", starlark.String(username))
	// Note: password is intentionally not exposed as an attribute for security
	return a, nil
}

// authBearerToken creates an auth.bearer_token configuration.
func authBearerToken(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("auth.bearer_token", args, kwargs)

	var envVar, tokenValue string
	if err := u.Optional("env", &envVar, ""); err != nil {
		return nil, err
	}
	if err := u.Optional("value", &tokenValue, ""); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	// Require exactly one of env or value
	if envVar == "" && tokenValue == "" {
		return nil, fmt.Errorf("auth.bearer_token: requires either 'env' or 'value' argument")
	}
	if envVar != "" && tokenValue != "" {
		return nil, fmt.Errorf("auth.bearer_token: cannot specify both 'env' and 'value'")
	}

	a := newAuthValue(AuthTypeBearerToken)
	a.EnvVar = envVar
	a.TokenValue = tokenValue
	if envVar != "" {
		a.Set("env", starlark.String(envVar))
	}
	// Note: token value is intentionally not exposed as an attribute for security
	return a, nil
}

// authHeader creates an auth.header configuration.
func authHeader(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	u := value.NewUnpacker("auth.header", args, kwargs)

	var name, headerValue string
	if err := u.Required("name", &name); err != nil {
		return nil, err
	}
	if err := u.Required("value", &headerValue); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	a := newAuthValue(AuthTypeHeader)
	a.HeaderName = name
	a.HeaderValue = headerValue
	a.Set("name", starlark.String(name))
	// Note: header value is intentionally not exposed as an attribute for security
	return a, nil
}

// AuthModule returns the auth Starlark module.
func AuthModule() starlark.StringDict {
	return starlark.StringDict{
		"basic":        starlark.NewBuiltin("auth.basic", authBasic),
		"bearer_token": starlark.NewBuiltin("auth.bearer_token", authBearerToken),
		"header":       starlark.NewBuiltin("auth.header", authHeader),
	}
}
