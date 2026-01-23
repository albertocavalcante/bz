package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// HTTPRegistry implements Registry for HTTP/HTTPS registries.
type HTTPRegistry struct {
	baseURL    string
	httpClient *http.Client
}

// HTTP client configuration.
const (
	httpTimeout = 30 * time.Second
)

// nginx autoindex entry types.
const (
	nginxTypeDirectory = "directory"
)

// NewHTTPRegistry creates a new HTTP-based registry.
func NewHTTPRegistry(baseURL string) *HTTPRegistry {
	// Normalize URL (remove trailing slash)
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &HTTPRegistry{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// Type returns the registry type identifier.
func (r *HTTPRegistry) Type() string {
	if strings.HasPrefix(r.baseURL, "https://") {
		return TypeHTTPS
	}
	return TypeHTTP
}

// String returns a human-readable representation.
func (r *HTTPRegistry) String() string {
	return r.baseURL
}

// GetMetadata fetches the metadata.json for a module.
func (r *HTTPRegistry) GetMetadata(ctx context.Context, module string) (*Metadata, error) {
	url := r.baseURL + "/" + MetadataPath(module)

	data, err := r.fetch(ctx, url)
	if err != nil {
		return nil, err
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}

	return &meta, nil
}

// GetModuleBazel fetches the MODULE.bazel content for a specific version.
func (r *HTTPRegistry) GetModuleBazel(ctx context.Context, module, version string) ([]byte, error) {
	// First verify module exists by checking metadata
	_, err := r.GetMetadata(ctx, module)
	if err != nil {
		return nil, err
	}

	url := r.baseURL + "/" + ModuleBazelPath(module, version)
	data, err := r.fetch(ctx, url)
	if err != nil {
		// If module exists but version doesn't, return ErrVersionNotFound
		if err == ErrModuleNotFound {
			return nil, ErrVersionNotFound
		}
		return nil, err
	}

	return data, nil
}

// ListModules returns all module names in the registry.
// It tries multiple strategies: index.json, HTML directory listing, nginx JSON autoindex.
func (r *HTTPRegistry) ListModules(ctx context.Context) ([]string, error) {
	// Strategy 1: Try index.json
	modules, err := r.listFromIndexJSON(ctx)
	if err == nil {
		return modules, nil
	}

	// Strategy 2: Try directory listing (HTML or nginx JSON)
	modules, err = r.listFromDirectoryListing(ctx)
	if err == nil {
		return modules, nil
	}

	return nil, ErrListingNotSupported
}

// listFromIndexJSON tries to fetch modules from index.json.
func (r *HTTPRegistry) listFromIndexJSON(ctx context.Context) ([]string, error) {
	url := r.baseURL + "/" + ModulesIndexPath()

	data, err := r.fetch(ctx, url)
	if err != nil {
		return nil, err
	}

	var modules []string
	if err := json.Unmarshal(data, &modules); err != nil {
		return nil, err
	}

	sort.Strings(modules)
	return modules, nil
}

// listFromDirectoryListing tries to parse directory listing (HTML or nginx JSON).
func (r *HTTPRegistry) listFromDirectoryListing(ctx context.Context) ([]string, error) {
	// Try with trailing slash first (common convention), then without
	urls := []string{
		r.baseURL + "/" + ModulesDir + "/",
		r.baseURL + "/" + ModulesDir,
	}

	for _, url := range urls {
		body, contentType, err := r.fetchDirectoryListing(ctx, url)
		if err != nil {
			continue // Try next URL
		}

		// Try nginx JSON autoindex
		if strings.Contains(contentType, "application/json") {
			return r.parseNginxAutoindex(body)
		}

		// Try HTML directory listing
		if strings.Contains(contentType, "text/html") {
			return r.parseHTMLDirectoryListing(body)
		}
	}

	return nil, ErrListingNotSupported
}

// fetchDirectoryListing fetches a URL and returns body and content-type if successful.
func (r *HTTPRegistry) fetchDirectoryListing(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return body, resp.Header.Get("Content-Type"), nil
}

// nginxAutoindexEntry represents an entry in nginx JSON autoindex.
type nginxAutoindexEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// parseNginxAutoindex parses nginx JSON autoindex format.
func (r *HTTPRegistry) parseNginxAutoindex(body []byte) ([]string, error) {
	var entries []nginxAutoindexEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}

	var modules []string
	for _, entry := range entries {
		if entry.Type == nginxTypeDirectory {
			modules = append(modules, entry.Name)
		}
	}

	sort.Strings(modules)
	return modules, nil
}

// htmlHrefPattern matches href attributes in anchor tags.
var htmlHrefPattern = regexp.MustCompile(`<a\s+href="([^"]+)/"`)

// parseHTMLDirectoryListing parses Apache/nginx HTML directory listing.
func (r *HTTPRegistry) parseHTMLDirectoryListing(body []byte) ([]string, error) {
	matches := htmlHrefPattern.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, ErrListingNotSupported
	}

	var modules []string
	for _, match := range matches {
		name := string(match[1])
		// Skip parent directory and hidden files
		if name == ".." || strings.HasPrefix(name, ".") {
			continue
		}
		modules = append(modules, name)
	}

	if len(modules) == 0 {
		return nil, ErrListingNotSupported
	}

	sort.Strings(modules)
	return modules, nil
}

// fetch performs an HTTP GET and returns the response body.
func (r *HTTPRegistry) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrModuleNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return body, nil
}

// Verify HTTPRegistry implements Registry.
var _ Registry = (*HTTPRegistry)(nil)
