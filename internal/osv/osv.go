// Package osv provides a client for querying the Open Source Vulnerabilities (OSV) API.
// It supports checking Bazel modules for known security vulnerabilities.
//
// Note: OSV does not have a "Bazel" ecosystem. This package maps known Bazel modules
// to their underlying ecosystems (e.g., rules_go -> Go, rules_python -> PyPI).
package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// EcosystemMapping maps Bazel module name prefixes to OSV ecosystems.
// OSV supported ecosystems include: Go, PyPI, crates.io, npm, Maven, NuGet, etc.
// See https://ossf.github.io/osv-schema/ for the full list.
var EcosystemMapping = map[string]string{
	// Go ecosystem mappings
	"rules_go":    "Go",
	"gazelle":     "Go",
	"com_github_": "Go", // Common Go modules from GitHub

	// Python ecosystem mappings
	"rules_python": "PyPI",
	"pip_":         "PyPI",

	// Rust ecosystem mappings
	"rules_rust": "crates.io",
	"crate_":     "crates.io",
	"crates_io_": "crates.io",

	// JavaScript/Node ecosystem mappings
	"rules_nodejs": "npm",
	"npm_":         "npm",

	// Java/JVM ecosystem mappings
	"rules_java":   "Maven",
	"rules_jvm_":   "Maven",
	"maven_":       "Maven",
	"rules_kotlin": "Maven",
	"rules_scala":  "Maven",

	// Other ecosystems
	"rules_ruby":   "RubyGems",
	"rules_dotnet": "NuGet",
	"rules_swift":  "SwiftURL",
	"rules_pkg":    "", // No direct mapping
	"rules_proto":  "", // No direct mapping
	"rules_cc":     "", // No direct mapping (C/C++ not in OSV)
}

// MapModuleToEcosystem attempts to map a Bazel module name to an OSV ecosystem.
// It returns the ecosystem name and a boolean indicating if a mapping was found.
// An empty ecosystem string with true means the module is known but has no OSV equivalent.
func MapModuleToEcosystem(moduleName string) (ecosystem string, known bool) {
	// Check for exact match first
	if eco, ok := EcosystemMapping[moduleName]; ok {
		return eco, true
	}

	// Check for prefix matches
	for prefix, eco := range EcosystemMapping {
		if strings.HasPrefix(moduleName, prefix) {
			return eco, true
		}
	}

	return "", false
}

// ExtractPackageName attempts to extract the underlying package name from a Bazel module.
// For example, "rules_go" might map to "golang.org/x/..." packages.
// Currently returns the module name as-is since Bazel modules don't always have
// a direct 1:1 mapping to package names.
func ExtractPackageName(moduleName string) string {
	// For now, return the module name as-is
	// In the future, this could be enhanced to extract actual package names
	// from extension tags or other sources
	return moduleName
}

// DefaultAPIURL is the default OSV API endpoint.
const DefaultAPIURL = "https://api.osv.dev/v1/query"

// ErrNoEcosystem is returned when a module cannot be mapped to an OSV ecosystem.
type ErrNoEcosystem struct {
	Module string
	Known  bool // true if module is known but has no OSV ecosystem mapping
}

func (e *ErrNoEcosystem) Error() string {
	if e.Known {
		return fmt.Sprintf("module %q has no OSV ecosystem equivalent", e.Module)
	}
	return fmt.Sprintf("unknown module %q: cannot determine OSV ecosystem (use --ecosystem flag to specify)", e.Module)
}

// Severity levels for vulnerabilities.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityUnknown  Severity = "UNKNOWN"
)

// SeverityOrder returns the severity level as an integer for comparison.
// Higher values indicate more severe vulnerabilities.
func (s Severity) Order() int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}

// ParseSeverity converts a string to a Severity, case-insensitively.
func ParseSeverity(s string) Severity {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return SeverityCritical
	case "HIGH":
		return SeverityHigh
	case "MEDIUM":
		return SeverityMedium
	case "LOW":
		return SeverityLow
	default:
		return SeverityUnknown
	}
}

// Vulnerability represents a single security vulnerability from OSV.
type Vulnerability struct {
	ID       string   `json:"id"`
	Summary  string   `json:"summary"`
	Details  string   `json:"details"`
	Severity Severity `json:"severity"`
	Aliases  []string `json:"aliases,omitempty"`
	Fixed    string   `json:"fixed,omitempty"`
	Link     string   `json:"link,omitempty"`
	Affected []string `json:"affected_versions,omitempty"`
}

// QueryRequest represents a request to the OSV API.
type QueryRequest struct {
	Package Package `json:"package"`
	Version string  `json:"version,omitempty"`
}

// Package identifies a package for vulnerability lookup.
type Package struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// QueryResponse represents the OSV API response.
type QueryResponse struct {
	Vulns []osvVuln `json:"vulns"`
}

// osvVuln is the internal representation matching the OSV API schema.
type osvVuln struct {
	ID         string        `json:"id"`
	Summary    string        `json:"summary"`
	Details    string        `json:"details"`
	Aliases    []string      `json:"aliases"`
	Severity   []osvSeverity `json:"severity"`
	Affected   []osvAffected `json:"affected"`
	References []osvRef      `json:"references"`
}

type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

type osvAffected struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Ranges   []osvRange `json:"ranges"`
	Versions []string   `json:"versions"`
}

type osvRange struct {
	Type   string     `json:"type"`
	Events []osvEvent `json:"events"`
}

type osvEvent struct {
	Introduced string `json:"introduced,omitempty"`
	Fixed      string `json:"fixed,omitempty"`
}

type osvRef struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Client is the interface for querying vulnerabilities.
type Client interface {
	// Query checks a package for known vulnerabilities.
	// The ecosystem parameter specifies the OSV ecosystem to query.
	// If ecosystem is empty, the client will attempt to map the module name
	// to an appropriate ecosystem using MapModuleToEcosystem.
	Query(ctx context.Context, name, version, ecosystem string) ([]Vulnerability, error)
}

// HTTPClient implements Client using the OSV HTTP API.
type HTTPClient struct {
	apiURL     string
	httpClient *http.Client
}

// ClientOption configures an HTTPClient.
type ClientOption func(*HTTPClient)

// WithAPIURL sets a custom API URL.
func WithAPIURL(url string) ClientOption {
	return func(c *HTTPClient) {
		c.apiURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *HTTPClient) {
		c.httpClient = hc
	}
}

// NewClient creates a new OSV API client.
func NewClient(opts ...ClientOption) *HTTPClient {
	c := &HTTPClient{
		apiURL:     DefaultAPIURL,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Query checks a package for known vulnerabilities.
// If ecosystem is empty, the method attempts to map the module name to an ecosystem.
// Returns ErrNoEcosystem if no ecosystem mapping exists for the module.
func (c *HTTPClient) Query(ctx context.Context, name, version, ecosystem string) ([]Vulnerability, error) {
	// If no ecosystem specified, try to map from module name
	if ecosystem == "" {
		mapped, known := MapModuleToEcosystem(name)
		if !known {
			return nil, &ErrNoEcosystem{Module: name}
		}
		if mapped == "" {
			// Known module but no OSV ecosystem (e.g., rules_cc)
			return nil, &ErrNoEcosystem{Module: name, Known: true}
		}
		ecosystem = mapped
	}

	req := QueryRequest{
		Package: Package{
			Name:      ExtractPackageName(name),
			Ecosystem: ecosystem,
		},
		Version: version,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("OSV API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var queryResp QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return convertVulns(queryResp.Vulns), nil
}

// convertVulns transforms OSV API vulnerabilities to our internal format.
func convertVulns(vulns []osvVuln) []Vulnerability {
	result := make([]Vulnerability, 0, len(vulns))
	for i := range vulns {
		v := &vulns[i]
		vuln := Vulnerability{
			ID:      v.ID,
			Summary: v.Summary,
			Details: v.Details,
			Aliases: v.Aliases,
		}

		// Extract severity from CVSS scores
		vuln.Severity = extractSeverity(v.Severity)

		// Find fixed version
		vuln.Fixed = findFixedVersion(v.Affected)

		// Find reference link
		for _, ref := range v.References {
			if ref.Type == "ADVISORY" || ref.Type == "WEB" {
				vuln.Link = ref.URL
				break
			}
		}
		if vuln.Link == "" && len(v.References) > 0 {
			vuln.Link = v.References[0].URL
		}

		result = append(result, vuln)
	}
	return result
}

// extractSeverity determines the severity from CVSS scores.
func extractSeverity(severities []osvSeverity) Severity {
	for _, s := range severities {
		if s.Type == "CVSS_V3" || s.Type == "CVSS_V2" {
			// Parse CVSS score to determine severity
			// CVSS V3 severity mapping:
			// 9.0-10.0: Critical
			// 7.0-8.9: High
			// 4.0-6.9: Medium
			// 0.1-3.9: Low
			return parseCVSSSeverity(s.Score)
		}
	}
	return SeverityUnknown
}

// parseCVSSSeverity extracts severity from a CVSS vector or score.
func parseCVSSSeverity(score string) Severity {
	// Look for severity indicator in CVSS vector string
	upper := strings.ToUpper(score)

	// Check for embedded severity in vector
	if strings.Contains(upper, "CRITICAL") {
		return SeverityCritical
	}
	if strings.Contains(upper, "HIGH") {
		return SeverityHigh
	}
	if strings.Contains(upper, "MEDIUM") {
		return SeverityMedium
	}
	if strings.Contains(upper, "LOW") {
		return SeverityLow
	}

	// Try to extract numeric score from CVSS vector
	// Format: CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H
	// Or just a numeric score like "9.8"
	var numScore float64
	if _, err := fmt.Sscanf(score, "%f", &numScore); err == nil {
		return scoreToSeverity(numScore)
	}

	return SeverityUnknown
}

// scoreToSeverity converts a numeric CVSS score to severity level.
func scoreToSeverity(score float64) Severity {
	switch {
	case score >= 9.0:
		return SeverityCritical
	case score >= 7.0:
		return SeverityHigh
	case score >= 4.0:
		return SeverityMedium
	case score > 0:
		return SeverityLow
	default:
		return SeverityUnknown
	}
}

// findFixedVersion extracts the fixed version from affected ranges.
func findFixedVersion(affected []osvAffected) string {
	for _, a := range affected {
		for _, r := range a.Ranges {
			for _, e := range r.Events {
				if e.Fixed != "" {
					return e.Fixed
				}
			}
		}
	}
	return ""
}

// MockClient is a mock implementation for testing.
type MockClient struct {
	Vulnerabilities map[string][]Vulnerability
	Error           error
	// SkipEcosystemCheck when true, bypasses ecosystem validation
	SkipEcosystemCheck bool
}

// Query returns mock vulnerabilities for testing.
// The ecosystem parameter is ignored in the mock client for backwards compatibility.
func (m *MockClient) Query(ctx context.Context, name, version, ecosystem string) ([]Vulnerability, error) {
	if m.Error != nil {
		return nil, m.Error
	}

	// Simulate ecosystem mapping behavior unless skipped
	if !m.SkipEcosystemCheck && ecosystem == "" {
		mapped, known := MapModuleToEcosystem(name)
		if !known {
			return nil, &ErrNoEcosystem{Module: name}
		}
		if mapped == "" {
			return nil, &ErrNoEcosystem{Module: name, Known: true}
		}
	}

	key := name + "@" + version
	if vulns, ok := m.Vulnerabilities[key]; ok {
		return vulns, nil
	}
	// Also check without version
	if vulns, ok := m.Vulnerabilities[name]; ok {
		return vulns, nil
	}
	return nil, nil
}
