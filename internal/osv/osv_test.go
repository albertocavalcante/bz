package osv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeverity_Order(t *testing.T) {
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
			assert.Equal(t, tt.order, tt.severity.Order())
		})
	}
}

func TestParseSeverity(t *testing.T) {
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
			assert.Equal(t, tt.expected, ParseSeverity(tt.input))
		})
	}
}

func TestScoreToSeverity(t *testing.T) {
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
			assert.Equal(t, tt.expected, scoreToSeverity(tt.score))
		})
	}
}

func TestHTTPClient_Query(t *testing.T) {
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

func TestMockClient_Query(t *testing.T) {
	mockVulns := []Vulnerability{
		{
			ID:       "GHSA-mock-1234",
			Summary:  "Mock vulnerability",
			Severity: SeverityHigh,
			Fixed:    "2.0.0",
		},
	}

	client := &MockClient{
		Vulnerabilities: map[string][]Vulnerability{
			"rules_go@0.50.0": mockVulns,
		},
	}

	// Query with matching key (rules_go is in the mapping)
	vulns, err := client.Query(context.Background(), "rules_go", "0.50.0", "")
	require.NoError(t, err)
	assert.Len(t, vulns, 1)
	assert.Equal(t, "GHSA-mock-1234", vulns[0].ID)

	// Query without match (rules_python is also in the mapping)
	vulns, err = client.Query(context.Background(), "rules_python", "1.0.0", "")
	require.NoError(t, err)
	assert.Nil(t, vulns)
}

func TestMockClient_Query_WithError(t *testing.T) {
	client := &MockClient{
		Error: assert.AnError,
	}

	vulns, err := client.Query(context.Background(), "rules_go", "1.0.0", "")
	require.Error(t, err)
	assert.Nil(t, vulns)
}

func TestMockClient_Query_ByNameOnly(t *testing.T) {
	mockVulns := []Vulnerability{
		{ID: "GHSA-all-versions"},
	}

	client := &MockClient{
		Vulnerabilities: map[string][]Vulnerability{
			"rules_go": mockVulns,
		},
	}

	// Query matches by name when no version-specific entry exists
	vulns, err := client.Query(context.Background(), "rules_go", "any-version", "")
	require.NoError(t, err)
	assert.Len(t, vulns, 1)
}

func TestNewClient_Options(t *testing.T) {
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
	client := NewClient()

	assert.Equal(t, DefaultAPIURL, client.apiURL)
	assert.Equal(t, http.DefaultClient, client.httpClient)
}

func TestMapModuleToEcosystem(t *testing.T) {
	tests := []struct {
		module    string
		ecosystem string
		known     bool
	}{
		// Exact matches
		{"rules_go", "Go", true},
		{"gazelle", "Go", true},
		{"rules_python", "PyPI", true},
		{"rules_rust", "crates.io", true},
		{"rules_nodejs", "npm", true},
		{"rules_java", "Maven", true},
		{"rules_kotlin", "Maven", true},
		{"rules_scala", "Maven", true},
		{"rules_ruby", "RubyGems", true},
		{"rules_dotnet", "NuGet", true},
		{"rules_swift", "SwiftURL", true},
		// Known but no OSV equivalent
		{"rules_cc", "", true},
		{"rules_pkg", "", true},
		{"rules_proto", "", true},
		// Prefix matches
		{"com_github_foo_bar", "Go", true},
		{"pip_install", "PyPI", true},
		{"crate_foo", "crates.io", true},
		{"crates_io_bar", "crates.io", true},
		{"npm_react", "npm", true},
		{"maven_junit", "Maven", true},
		{"rules_jvm_external", "Maven", true},
		// Unknown modules
		{"unknown_module", "", false},
		{"my_custom_rules", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			eco, known := MapModuleToEcosystem(tt.module)
			assert.Equal(t, tt.ecosystem, eco, "ecosystem mismatch for %s", tt.module)
			assert.Equal(t, tt.known, known, "known mismatch for %s", tt.module)
		})
	}
}

func TestErrNoEcosystem(t *testing.T) {
	// Unknown module
	err := &ErrNoEcosystem{Module: "unknown_module"}
	assert.Contains(t, err.Error(), "unknown module")
	assert.Contains(t, err.Error(), "unknown_module")
	assert.Contains(t, err.Error(), "--ecosystem")

	// Known module with no OSV equivalent
	err = &ErrNoEcosystem{Module: "rules_cc", Known: true}
	assert.Contains(t, err.Error(), "rules_cc")
	assert.Contains(t, err.Error(), "no OSV ecosystem equivalent")
}

func TestHTTPClient_Query_EcosystemMapping(t *testing.T) {
	// Test that the client correctly maps modules to ecosystems
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req QueryRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify the ecosystem was correctly mapped
		assert.Equal(t, "Go", req.Package.Ecosystem)

		json.NewEncoder(w).Encode(QueryResponse{Vulns: nil})
	}))
	defer server.Close()

	client := NewClient(WithAPIURL(server.URL))

	// rules_go should map to Go ecosystem
	_, err := client.Query(context.Background(), "rules_go", "0.50.0", "")
	require.NoError(t, err)
}

func TestHTTPClient_Query_UnknownModule(t *testing.T) {
	client := NewClient()

	// Unknown module should return ErrNoEcosystem
	_, err := client.Query(context.Background(), "unknown_module", "1.0.0", "")
	require.Error(t, err)

	var noEcoErr *ErrNoEcosystem
	require.ErrorAs(t, err, &noEcoErr)
	assert.Equal(t, "unknown_module", noEcoErr.Module)
	assert.False(t, noEcoErr.Known)
}

func TestHTTPClient_Query_KnownModuleNoOSV(t *testing.T) {
	client := NewClient()

	// rules_cc is known but has no OSV equivalent
	_, err := client.Query(context.Background(), "rules_cc", "0.0.9", "")
	require.Error(t, err)

	var noEcoErr *ErrNoEcosystem
	require.ErrorAs(t, err, &noEcoErr)
	assert.Equal(t, "rules_cc", noEcoErr.Module)
	assert.True(t, noEcoErr.Known)
}

func TestHTTPClient_Query_ExplicitEcosystem(t *testing.T) {
	// Test that explicit ecosystem overrides mapping
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req QueryRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Should use the explicitly provided ecosystem, not the mapping
		assert.Equal(t, "PyPI", req.Package.Ecosystem)

		json.NewEncoder(w).Encode(QueryResponse{Vulns: nil})
	}))
	defer server.Close()

	client := NewClient(WithAPIURL(server.URL))

	// Even though rules_go maps to Go, explicit PyPI should be used
	_, err := client.Query(context.Background(), "rules_go", "0.50.0", "PyPI")
	require.NoError(t, err)
}
