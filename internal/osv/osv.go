// Package osv provides a client for querying the Open Source Vulnerabilities (OSV) API.
//
// OSV has no Bazel ecosystem: records are filed under upstream package names
// (https://ossf.github.io/osv-schema/#affectedpackage-field), so the caller must
// supply one; a Bazel module name is generally not an upstream package name.
package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultAPIURL is the default OSV API endpoint.
const DefaultAPIURL = "https://api.osv.dev/v1/query"

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
	// name is the OSV package name within the given, non-empty ecosystem.
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

// Query checks a package for known vulnerabilities. The ecosystem is required.
func (c *HTTPClient) Query(ctx context.Context, name, version, ecosystem string) ([]Vulnerability, error) {
	if ecosystem == "" {
		return nil, errors.New("ecosystem is required")
	}

	req := QueryRequest{
		Package: Package{
			Name:      name,
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
