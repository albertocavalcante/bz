package osv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeverity_Order(t *testing.T) {
	t.Parallel()
	tests := []struct {
		severity Severity
		order    int
	}{
		{SeverityCritical, 4},
		{SeverityHigh, 3},
		{SeverityMedium, 2},
		{SeverityLow, 1},
		{SeverityUnknown, 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.order, tt.severity.Order())
		})
	}
}

func TestParseSeverity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected Severity
	}{
		{"critical", SeverityCritical},
		{"CRITICAL", SeverityCritical},
		{"Critical", SeverityCritical},
		{"high", SeverityHigh},
		{"HIGH", SeverityHigh},
		{"medium", SeverityMedium},
		{"MEDIUM", SeverityMedium},
		{"low", SeverityLow},
		{"LOW", SeverityLow},
		{"unknown", SeverityUnknown},
		{"", SeverityUnknown},
		{"foo", SeverityUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ParseSeverity(tt.input))
		})
	}
}

func TestScoreToSeverity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		score    float64
		expected Severity
	}{
		{10.0, SeverityCritical},
		{9.0, SeverityCritical},
		{8.9, SeverityHigh},
		{7.0, SeverityHigh},
		{6.9, SeverityMedium},
		{4.0, SeverityMedium},
		{3.9, SeverityLow},
		{0.1, SeverityLow},
		{0.0, SeverityUnknown},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, scoreToSeverity(tt.score))
		})
	}
}

func TestHTTPClient_Query(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		response    QueryResponse
		statusCode  int
		wantVulns   int
		wantErr     bool
		errContains string
	}{
		{
			name: "no vulnerabilities",
			response: QueryResponse{
				Vulns: nil,
			},
			statusCode: http.StatusOK,
			wantVulns:  0,
		},
		{
			name: "single vulnerability",
			response: QueryResponse{
				Vulns: []osvVuln{
					{
						ID:      "GHSA-1234-5678-9abc",
						Summary: "Test vulnerability",
						Details: "This is a test vulnerability",
						Severity: []osvSeverity{
							{Type: "CVSS_V3", Score: "9.8"},
						},
						Affected: []osvAffected{
							{
								Ranges: []osvRange{
									{
										Events: []osvEvent{
											{Introduced: "0"},
											{Fixed: "1.0.1"},
										},
									},
								},
							},
						},
						References: []osvRef{
							{Type: "ADVISORY", URL: "https://example.com/advisory"},
						},
					},
				},
			},
			statusCode: http.StatusOK,
			wantVulns:  1,
		},
		{
			name: "multiple vulnerabilities",
			response: QueryResponse{
				Vulns: []osvVuln{
					{
						ID:      "GHSA-1111-2222-3333",
						Summary: "First vulnerability",
					},
					{
						ID:      "GHSA-4444-5555-6666",
						Summary: "Second vulnerability",
					},
				},
			},
			statusCode: http.StatusOK,
			wantVulns:  2,
		},
		{
			name:        "server error",
			statusCode:  http.StatusInternalServerError,
			wantErr:     true,
			errContains: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Verify request body
				var req QueryRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)
				assert.Equal(t, "test-module", req.Package.Name)
				assert.Equal(t, "Go", req.Package.Ecosystem) // Now expects Go since we specify it
				assert.Equal(t, "1.0.0", req.Version)

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			client := NewClient(WithAPIURL(server.URL))
			// Explicitly specify ecosystem since "test-module" is not in the mapping
			vulns, err := client.Query(context.Background(), "test-module", "1.0.0", "Go")

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Len(t, vulns, tt.wantVulns)
		})
	}
}

func TestHTTPClient_Query_ParsesVulnerabilityDetails(t *testing.T) {
	t.Parallel()
	response := QueryResponse{
		Vulns: []osvVuln{
			{
				ID:      "GHSA-test-1234-5678",
				Summary: "Remote code execution vulnerability",
				Details: "A detailed description of the vulnerability",
				Aliases: []string{"CVE-2024-1234"},
				Severity: []osvSeverity{
					{Type: "CVSS_V3", Score: "9.8"},
				},
				Affected: []osvAffected{
					{
						Ranges: []osvRange{
							{
								Events: []osvEvent{
									{Introduced: "0.5.0"},
									{Fixed: "1.2.3"},
								},
							},
						},
					},
				},
				References: []osvRef{
					{Type: "ADVISORY", URL: "https://github.com/security/advisories/GHSA-test"},
					{Type: "WEB", URL: "https://example.com/details"},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(WithAPIURL(server.URL))
	vulns, err := client.Query(context.Background(), "test-module", "1.0.0", "Go")

	require.NoError(t, err)
	require.Len(t, vulns, 1)

	v := vulns[0]
	assert.Equal(t, "GHSA-test-1234-5678", v.ID)
	assert.Equal(t, "Remote code execution vulnerability", v.Summary)
	assert.Equal(t, "A detailed description of the vulnerability", v.Details)
	assert.Equal(t, []string{"CVE-2024-1234"}, v.Aliases)
	assert.Equal(t, SeverityCritical, v.Severity)
	assert.Equal(t, "1.2.3", v.Fixed)
	assert.Equal(t, "https://github.com/security/advisories/GHSA-test", v.Link)
}

func TestHTTPClient_Query_SeverityParsing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		severities  []osvSeverity
		expectedSev Severity
	}{
		{
			name:        "no severity",
			severities:  nil,
			expectedSev: SeverityUnknown,
		},
		{
			name: "CVSS_V3 critical score",
			severities: []osvSeverity{
				{Type: "CVSS_V3", Score: "9.8"},
			},
			expectedSev: SeverityCritical,
		},
		{
			name: "CVSS_V3 high score",
			severities: []osvSeverity{
				{Type: "CVSS_V3", Score: "7.5"},
			},
			expectedSev: SeverityHigh,
		},
		{
			name: "CVSS_V3 medium score",
			severities: []osvSeverity{
				{Type: "CVSS_V3", Score: "5.5"},
			},
			expectedSev: SeverityMedium,
		},
		{
			name: "CVSS_V3 low score",
			severities: []osvSeverity{
				{Type: "CVSS_V3", Score: "2.0"},
			},
			expectedSev: SeverityLow,
		},
		{
			name: "CVSS_V2 fallback",
			severities: []osvSeverity{
				{Type: "CVSS_V2", Score: "8.0"},
			},
			expectedSev: SeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			response := QueryResponse{
				Vulns: []osvVuln{
					{
						ID:       "GHSA-test",
						Severity: tt.severities,
					},
				},
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			client := NewClient(WithAPIURL(server.URL))
			vulns, err := client.Query(context.Background(), "test", "1.0.0", "Go")

			require.NoError(t, err)
			require.Len(t, vulns, 1)
			assert.Equal(t, tt.expectedSev, vulns[0].Severity)
		})
	}
}

func TestNewClient_Options(t *testing.T) {
	t.Parallel()
	customHTTP := &http.Client{}
	customURL := "https://custom.osv.dev/api"

	client := NewClient(
		WithAPIURL(customURL),
		WithHTTPClient(customHTTP),
	)

	assert.Equal(t, customURL, client.apiURL)
	assert.Equal(t, customHTTP, client.httpClient)
}

func TestNewClient_Defaults(t *testing.T) {
	t.Parallel()
	client := NewClient()

	assert.Equal(t, DefaultAPIURL, client.apiURL)
	assert.Equal(t, http.DefaultClient, client.httpClient)
}

func TestHTTPClient_Query_SendsPackageAsGiven(t *testing.T) {
	t.Parallel()
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	_, err := NewClient(WithAPIURL(server.URL)).Query(context.Background(), "rules_go", "0.50.0", "PyPI")

	require.NoError(t, err)
	assert.JSONEq(t, `{"package":{"name":"rules_go","ecosystem":"PyPI"},"version":"0.50.0"}`, got)
}

func TestHTTPClient_Query_RequiresEcosystem(t *testing.T) {
	t.Parallel()

	_, err := NewClient(WithAPIURL("http://127.0.0.1:1")).Query(context.Background(), "rules_go", "0.50.0", "")

	require.EqualError(t, err, "ecosystem is required")
}

func TestParseEcosystem(t *testing.T) {
	t.Parallel()
	const tail = "bz's list is from the OSV schema of 2026-11-24 (see https://ossf.github.io/osv-schema/#affectedpackage-field); if the spelling is right and OSV added it since, pass --ecosystem-unverified"
	tests := []struct {
		name, in, want, wantErr string
	}{
		{"canonical", "Go", "Go", ""},
		{"lowercase", "go", "Go", ""},
		{"mixed case", "PYPI", "PyPI", ""},
		{"crates", "CRATES.IO", "crates.io", ""},
		{"release suffix kept", "debian:12", "Debian:12", ""},
		{"typo", "Mavn", "", `unknown OSV ecosystem "Mavn": did you mean "Maven"? ` + tail},
		{"unknown", "bazel", "", `unknown OSV ecosystem "bazel": ` + tail},
		{"long value is truncated", strings.Repeat("x", 100), "", `unknown OSV ecosystem "` + strings.Repeat("x", 64) + `…": ` + tail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseEcosystem(tt.in)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
