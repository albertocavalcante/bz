package publisher

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// HTTP client configuration.
const (
	httpTimeout = 30 * time.Second
)

// Auth types for HTTP authentication.
const (
	AuthTypeBasic  = "basic"
	AuthTypeBearer = "bearer"
	AuthTypeHeader = "header"
)

// Auth contains authentication configuration for HTTP requests.
type Auth struct {
	// Type is the authentication method: "basic", "bearer", or "header".
	Type string

	// Username for basic auth.
	Username string

	// Password for basic auth.
	Password string

	// Token for bearer auth.
	Token string

	// EnvVar for bearer auth (environment variable containing the token).
	EnvVar string

	// HeaderName for custom header auth.
	HeaderName string

	// HeaderValue for custom header auth.
	HeaderValue string
}

// HTTPPublisher writes module files via HTTP PUT requests.
type HTTPPublisher struct {
	baseURL    string
	httpClient *http.Client
	auth       *Auth
}

// NewHTTPPublisher creates a new HTTP PUT publisher.
// The baseURL is the root URL where modules will be written.
func NewHTTPPublisher(baseURL string, auth *Auth) (*HTTPPublisher, error) {
	// Normalize URL (remove trailing slash)
	baseURL = strings.TrimSuffix(baseURL, "/")

	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	return &HTTPPublisher{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
		auth: auth,
	}, nil
}

// Put sends an HTTP PUT request to write content.
func (p *HTTPPublisher) Put(ctx context.Context, filePath string, content []byte) error {
	url := p.baseURL + "/" + filePath

	// Try to create parent directories via MKCOL (WebDAV)
	if err := p.ensureDirectories(ctx, filePath); err != nil {
		// Ignore errors - not all servers support MKCOL
		_ = err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set content type based on file extension
	req.Header.Set("Content-Type", p.contentType(filePath))

	// Add authentication
	p.addAuth(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", url, err)
	}
	defer resp.Body.Close()

	// Accept 2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PUT %s: HTTP %d", url, resp.StatusCode)
	}

	return nil
}

// Exists checks if a path exists via HTTP HEAD request.
func (p *HTTPPublisher) Exists(ctx context.Context, filePath string) (bool, error) {
	url := p.baseURL + "/" + filePath

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}

	p.addAuth(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("HEAD %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}

	return false, fmt.Errorf("HEAD %s: HTTP %d", url, resp.StatusCode)
}

// Finalize is a no-op for HTTP publisher.
func (p *HTTPPublisher) Finalize(_ context.Context, _ string) error {
	return nil
}

// Type returns the publisher type identifier.
func (p *HTTPPublisher) Type() string {
	return TypeHTTPPut
}

// Close releases any resources (no-op for HTTP publisher).
func (p *HTTPPublisher) Close() error {
	return nil
}

// addAuth adds authentication headers to a request.
func (p *HTTPPublisher) addAuth(req *http.Request) {
	if p.auth == nil {
		return
	}

	switch p.auth.Type {
	case AuthTypeBasic:
		req.SetBasicAuth(p.auth.Username, p.auth.Password)

	case AuthTypeBearer:
		token := p.auth.Token
		if token == "" && p.auth.EnvVar != "" {
			token = os.Getenv(p.auth.EnvVar)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

	case AuthTypeHeader:
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
		url := p.baseURL + "/" + dir

		req, err := http.NewRequestWithContext(ctx, "MKCOL", url, nil)
		if err != nil {
			return err
		}

		p.addAuth(req)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return err
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
