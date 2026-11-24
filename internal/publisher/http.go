package publisher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/albertocavalcante/bz/internal/config/modules"
	"github.com/albertocavalcante/bz/internal/coord"
)

// HTTP client configuration.
const (
	httpTimeout = 30 * time.Second
)

// Auth contains authentication configuration for HTTP requests.
// Auth type constants are defined in the modules package
// (modules.AuthTypeBasic, modules.AuthTypeBearerToken, modules.AuthTypeHeader).
type Auth struct {
	// Type is the authentication method: "basic", "bearer_token", or "header".
	Type string

	// Username for basic auth.
	Username string

	// Password for basic auth.
	Password string

	// TokenValue for bearer auth.
	TokenValue string

	// EnvVar for bearer auth (environment variable containing the token).
	EnvVar string

	// HeaderName for custom header auth.
	HeaderName string

	// HeaderValue for custom header auth.
	HeaderValue string
}

// HTTPPublisher writes module files via HTTP PUT requests.
type HTTPPublisher struct {
	base       *neturl.URL // parsed once; no query or fragment
	basePath   string      // base.EscapedPath() without trailing slashes
	httpClient *http.Client
	transport  *http.Transport // owned by this publisher; released by Close
	auth       *Auth
}

// NewHTTPPublisher creates a new HTTP PUT publisher.
// The baseURL is the root URL where modules will be written.
// It must be an absolute http or https URL without a query or fragment;
// trailing slashes are ignored. Userinfo is allowed (net/http sends it as
// basic auth) but is redacted from every error message.
func NewHTTPPublisher(baseURL string, auth *Auth) (*HTTPPublisher, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	u, err := neturl.Parse(baseURL)
	if err != nil {
		var uerr *neturl.Error
		if errors.As(err, &uerr) {
			err = uerr.Err // drop the raw URL, which may carry a password
		}
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid base URL %s: scheme must be http or https", u.Redacted())
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid base URL %s: missing host", u.Redacted())
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(baseURL, "#") {
		return nil, fmt.Errorf("invalid base URL %s: it must not contain a query or fragment", redactedBase(u))
	}

	// Each publisher owns its transport. The zero-value http.Client falls
	// back to the process-wide http.DefaultTransport, which anything in the
	// process can flush with CloseIdleConnections (httptest.Server.Close
	// does), failing in-flight requests with "http: CloseIdleConnections
	// called". Cloning keeps the default dialer, proxy and TLS settings.
	transport := http.DefaultTransport.(*http.Transport).Clone()

	return &HTTPPublisher{
		base:     u,
		basePath: strings.TrimRight(u.EscapedPath(), "/"),
		httpClient: &http.Client{
			Timeout:   httpTimeout,
			Transport: transport,
		},
		transport: transport,
		auth:      auth,
	}, nil
}

// Put sends an HTTP PUT request to write content.
// The path must be a valid registry path (see coord.ValidateRegistryPath);
// invalid paths fail before any request is sent.
func (p *HTTPPublisher) Put(ctx context.Context, filePath string, content []byte) error {
	u, err := p.url(filePath)
	if err != nil {
		return err
	}

	// Try to create parent directories via MKCOL (WebDAV)
	if err := p.ensureDirectories(ctx, filePath); err != nil {
		// Ignore errors - not all servers support MKCOL
		_ = err
	}

	req, err := newRequest(ctx, http.MethodPut, u, bytes.NewReader(content))
	if err != nil {
		return err
	}

	// Set content type based on file extension
	req.Header.Set("Content-Type", p.contentType(filePath))

	// Add authentication
	p.addAuth(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", u.Redacted(), err)
	}
	defer resp.Body.Close()

	// Accept 2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PUT %s: HTTP %d", u.Redacted(), resp.StatusCode)
	}

	return nil
}

// Exists checks if a path exists via HTTP HEAD request.
func (p *HTTPPublisher) Exists(ctx context.Context, filePath string) (bool, error) {
	u, err := p.url(filePath)
	if err != nil {
		return false, err
	}

	req, err := newRequest(ctx, http.MethodHead, u, nil)
	if err != nil {
		return false, err
	}

	p.addAuth(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("HEAD %s: %w", u.Redacted(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}

	return false, fmt.Errorf("HEAD %s: HTTP %d", u.Redacted(), resp.StatusCode)
}

// Finalize is a no-op for HTTP publisher.
func (p *HTTPPublisher) Finalize(_ context.Context, _ string) error {
	return nil
}

// Type returns the publisher type identifier.
func (p *HTTPPublisher) Type() string {
	return TypeHTTPPut
}

// Close releases this publisher's idle connections. It never touches the
// process-wide default transport or other publishers.
func (p *HTTPPublisher) Close() error {
	p.transport.CloseIdleConnections()
	return nil
}

// url validates a registry path and appends it to the base URL. Each
// segment is also escaped with url.PathEscape, so even a segment that slipped
// past validation is sent as one literal path segment and cannot introduce a
// query, fragment, or decoded "..".
func (p *HTTPPublisher) url(filePath string) (*neturl.URL, error) {
	if err := coord.ValidateRegistryPath(filePath); err != nil {
		return nil, err
	}
	return p.joinURL(filePath)
}

// joinURL appends a slash-separated path to the base URL's path, escaping
// each segment. Escaping leaves "." and ".." unchanged, so those (and empty
// segments) are refused outright. Path and RawPath are both set so the
// escaping survives serialization.
func (p *HTTPPublisher) joinURL(rel string) (*neturl.URL, error) {
	segs := strings.Split(rel, "/")
	for i, s := range segs {
		if s == "" || s == "." || s == ".." {
			return nil, fmt.Errorf("%w: %q", coord.ErrInvalidPath, rel)
		}
		segs[i] = neturl.PathEscape(s)
	}
	escaped := p.basePath + "/" + strings.Join(segs, "/")
	decoded, err := neturl.PathUnescape(escaped)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", coord.ErrInvalidPath, rel)
	}
	u := *p.base
	u.Path = decoded
	u.RawPath = escaped
	return &u, nil
}

// newRequest builds a request for u. Errors name the redacted URL and never
// echo the raw one.
func newRequest(ctx context.Context, method string, u *neturl.URL, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		var uerr *neturl.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return nil, fmt.Errorf("%s %s: create request: %w", method, u.Redacted(), err)
	}
	return req, nil
}

// redactedBase renders a base URL for error messages without its password,
// query or fragment.
func redactedBase(u *neturl.URL) string {
	c := *u
	c.RawQuery, c.ForceQuery, c.Fragment, c.RawFragment = "", false, "", ""
	return c.Redacted()
}

// addAuth adds authentication headers to a request.
func (p *HTTPPublisher) addAuth(req *http.Request) {
	if p.auth == nil {
		return
	}

	switch p.auth.Type {
	case modules.AuthTypeBasic:
		req.SetBasicAuth(p.auth.Username, p.auth.Password)

	case modules.AuthTypeBearerToken:
		token := p.auth.TokenValue
		if token == "" && p.auth.EnvVar != "" {
			token = os.Getenv(p.auth.EnvVar)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

	case modules.AuthTypeHeader:
		if p.auth.HeaderName != "" && p.auth.HeaderValue != "" {
			req.Header.Set(p.auth.HeaderName, p.auth.HeaderValue)
		}
	}
}

// contentType returns the MIME type for a file based on its extension.
func (p *HTTPPublisher) contentType(filePath string) string {
	switch {
	case strings.HasSuffix(filePath, ".json"):
		return "application/json"
	case strings.HasSuffix(filePath, ".bazel"):
		return "text/plain"
	case strings.HasSuffix(filePath, ".patch"):
		return "text/x-patch"
	default:
		return "application/octet-stream"
	}
}

// ensureDirectories tries to create parent directories via WebDAV MKCOL.
func (p *HTTPPublisher) ensureDirectories(ctx context.Context, filePath string) error {
	// Get all parent directories
	dirs := p.parentDirs(filePath)

	for _, dir := range dirs {
		u, err := p.joinURL(dir) // dir is a prefix of a path already validated by Put
		if err != nil {
			return err
		}

		req, err := newRequest(ctx, "MKCOL", u, nil)
		if err != nil {
			return err
		}

		p.addAuth(req)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("MKCOL %s: %w", u.Redacted(), err)
		}
		_ = resp.Body.Close()

		// Ignore 405 (Method Not Allowed) and 409 (Conflict - already exists)
		// Continue on success or if directory already exists
	}

	return nil
}

// parentDirs returns all parent directories for a path.
func (p *HTTPPublisher) parentDirs(filePath string) []string {
	var dirs []string
	dir := path.Dir(filePath)

	for dir != "." && dir != "/" {
		dirs = append(dirs, dir)
		dir = path.Dir(dir)
	}

	// Reverse so parents come before children
	for i, j := 0, len(dirs)-1; i < j; i, j = i+1, j-1 {
		dirs[i], dirs[j] = dirs[j], dirs[i]
	}

	return dirs
}

// Verify HTTPPublisher implements Publisher.
var _ Publisher = (*HTTPPublisher)(nil)
