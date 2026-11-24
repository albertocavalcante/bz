package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/osv"
	"github.com/albertocavalcante/bz/internal/testutil"
)

const (
	sevenModules = `module(name = "app", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "gazelle", version = "0.38.0")
bazel_dep(name = "rules_python", version = "0.34.0")
bazel_dep(name = "abseil-cpp", version = "20240116.2")
bazel_dep(name = "protobuf", version = "27.1")
bazel_dep(name = "rules_cc", version = "0.0.9")
bazel_dep(name = "zlib", version = "1.3.1")
`
	unmappedModules = `module(name = "app", version = "1.0.0")

bazel_dep(name = "abseil-cpp", version = "20240116.2")
bazel_dep(name = "protobuf", version = "27.1")
bazel_dep(name = "rules_cc", version = "0.0.9")
bazel_dep(name = "zlib", version = "1.3.1")
`
	mappedModules = `module(name = "app", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "gazelle", version = "0.38.0")
`
)

// osvServer starts a fake OSV endpoint answering with status and body, and
// points the audit command at it. It returns a counter of requests served.
func osvServer(t *testing.T, status int, body string) *atomic.Int32 {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)

	old := osvClient
	osvClient = osv.NewClient(osv.WithAPIURL(srv.URL))
	t.Cleanup(func() { osvClient = old })
	return &hits
}

func auditProject(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MODULE.bazel"), []byte(content), 0o644))
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	for _, k := range []string{"BZ_OFFLINE", "BZ_PREFER_OFFLINE", "BZ_REGISTRY", "BZ_DISABLE_COMMANDS"} {
		t.Setenv(k, "")
	}
}

func TestAudit_MixedProjectIsNotClean(t *testing.T) {
	auditProject(t, sevenModules)
	osvServer(t, http.StatusOK, `{}`)

	stdout, stderr, code := runCLI(t, "audit")

	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, "Checked 3 of 7 direct dependencies")
	for _, m := range []string{"abseil-cpp@20240116.2", "protobuf@27.1", "rules_cc@0.0.9", "zlib@1.3.1"} {
		assert.Contains(t, stdout, m)
	}
	assert.Contains(t, stdout, "No vulnerabilities found in the 3 direct dependencies that could be checked; 4 direct dependencies were not checked.")
	assert.NotContains(t, strings.ReplaceAll(stdout, "found in the 3 modules", ""), "No vulnerabilities found\n")
	assert.Equal(t, 1, countErrorLines(stderr), stderr)
	assert.Contains(t, stderr, "4 of 7 direct dependencies were not checked")
}

func TestAudit_AllUnchecked(t *testing.T) {
	auditProject(t, unmappedModules)
	hits := osvServer(t, http.StatusOK, `{}`)

	stdout, stderr, code := runCLI(t, "audit")

	assert.Equal(t, 1, code)
	assert.Zero(t, hits.Load())
	assert.Contains(t, stdout, "Checked 0 of 4 direct dependencies")
	assert.Contains(t, stdout, "No direct dependency could be checked")
	assert.NotContains(t, stdout, "No vulnerabilities found")
	assert.Equal(t, 1, countErrorLines(stderr), stderr)
}

func TestAudit_AllowUnchecked(t *testing.T) {
	auditProject(t, sevenModules)
	osvServer(t, http.StatusOK, `{}`)

	stdout, stderr, code := runCLI(t, "audit", "--allow-unchecked")

	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "Checked 3 of 7 direct dependencies")
	assert.Contains(t, stdout, "Not checked (4):")
	assert.Contains(t, stdout, "zlib@1.3.1: no known OSV ecosystem mapping")
	assert.Contains(t, stdout, "rules_cc@0.0.9: no OSV equivalent")
	assert.Contains(t, stdout, "4 direct dependencies were not checked")
	assert.Empty(t, stderr)
}

func TestAudit_AllowUncheckedStillFailsOnVulnerabilities(t *testing.T) {
	auditProject(t, sevenModules)
	osvServer(t, http.StatusOK, `{"vulns":[{"id":"GHSA-x","summary":"bad"}]}`)

	stdout, stderr, code := runCLI(t, "audit", "--allow-unchecked")

	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, "GHSA-x")
	assert.Equal(t, 1, countErrorLines(stderr), stderr)
	assert.Contains(t, stderr, "vulnerabilities found")
}

func TestAudit_AllQueriesFail(t *testing.T) {
	auditProject(t, mappedModules)
	osvServer(t, http.StatusInternalServerError, "boom")

	for _, allow := range []bool{false, true} {
		args := []string{"audit"}
		if allow {
			args = append(args, "--allow-unchecked")
		}
		stdout, stderr, code := runCLI(t, args...)

		assert.Equal(t, 1, code, "allow-unchecked=%v", allow)
		assert.Contains(t, stdout, "Checked 0 of 2 direct dependencies")
		assert.Contains(t, stdout, "query failed:")
		assert.NotContains(t, stdout, "No vulnerabilities found")
		assert.Equal(t, 1, countErrorLines(stderr), stderr)
		assert.Contains(t, stderr, "OSV queries failed for 2 of 2 direct dependencies")
	}
}

func TestAudit_AllQueriesFailJSON(t *testing.T) {
	auditProject(t, mappedModules)
	osvServer(t, http.StatusInternalServerError, "boom")

	stdout, stderr, code := runCLI(t, "audit", "--json")

	assert.Equal(t, 1, code)
	assert.Equal(t, 1, countErrorLines(stderr), stderr)

	var got AuditResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &got), stdout)
	assert.Equal(t, AuditSummary{Scope: "direct", Total: 2, Checked: 0, Unchecked: 2, Failed: 2}, got.Summary)
	require.Len(t, got.Unchecked, 2)
	assert.Equal(t, "rules_go", got.Unchecked[0].Module)
	assert.Equal(t, "0.50.1", got.Unchecked[0].Version)
	assert.Contains(t, got.Unchecked[0].Reason, "query failed:")
	assert.Equal(t, 2, got.Total)
	assert.NotNil(t, got.Vulnerabilities)
}

func TestAudit_AllUncheckedJSON(t *testing.T) {
	auditProject(t, unmappedModules)
	osvServer(t, http.StatusOK, `{}`)

	stdout, _, code := runCLI(t, "audit", "--json")

	assert.Equal(t, 1, code)
	var got AuditResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &got), stdout)
	assert.Equal(t, AuditSummary{Scope: "direct", Total: 4, Checked: 0, Unchecked: 4, Failed: 0}, got.Summary)
	assert.Len(t, got.Unchecked, 4)

	stdout, stderr, code := runCLI(t, "audit", "--json", "--allow-unchecked")
	assert.Equal(t, 0, code, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &got), stdout)
	assert.Equal(t, 4, got.Summary.Unchecked)
}

func TestAudit_PartialFailure(t *testing.T) {
	auditProject(t, mappedModules)
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) == 1 {
			fmt.Fprint(w, `{}`)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	old := osvClient
	osvClient = osv.NewClient(osv.WithAPIURL(srv.URL))
	t.Cleanup(func() { osvClient = old })

	stdout, stderr, code := runCLI(t, "audit", "--allow-unchecked")

	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, "Checked 1 of 2 direct dependencies")
	assert.NotContains(t, stdout, "No vulnerabilities found\n")
	assert.Equal(t, 1, countErrorLines(stderr), stderr)
	assert.Contains(t, stderr, "OSV queries failed for 1 of 2 direct dependencies")
}

func TestAudit_FullyCheckedAndClean(t *testing.T) {
	auditProject(t, mappedModules)
	osvServer(t, http.StatusOK, `{}`)

	stdout, stderr, code := runCLI(t, "audit")

	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "Checked 2 of 2 direct dependencies")
	assert.Contains(t, stdout, "No vulnerabilities found")
	assert.NotContains(t, stdout, "not checked")
	assert.Empty(t, stderr)

	stdout, _, code = runCLI(t, "audit", "--json")
	assert.Equal(t, 0, code)
	var got AuditResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &got), stdout)
	assert.Equal(t, AuditSummary{Scope: "direct", Total: 2, Checked: 2}, got.Summary)
	assert.Empty(t, got.Unchecked)
	assert.Contains(t, stdout, `"unchecked": []`)
}

func TestAudit_UnreachableRegistryDoesNotReportClean(t *testing.T) {
	auditProject(t, unmappedModules)
	osvServer(t, http.StatusOK, `{}`)

	stdout, stderr, code := runCLI(t, "audit", "--registry", "http://127.0.0.1:1")

	assert.Equal(t, 1, code)
	assert.NotContains(t, stdout, "No vulnerabilities found")
	assert.Equal(t, 1, countErrorLines(stderr), stderr)
}

func TestAudit_FileRegistryProject(t *testing.T) {
	reg := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}},
	})
	auditProject(t, mappedModules)
	osvServer(t, http.StatusOK, `{}`)

	stdout, stderr, code := runCLI(t, "audit", "--registry", "file://"+reg)

	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "Checked 2 of 2 direct dependencies")
}

func TestAudit_InvalidSeverity(t *testing.T) {
	auditProject(t, mappedModules)
	hits := osvServer(t, http.StatusOK, `{}`)

	stdout, stderr, code := runCLI(t, "audit", "--severity=bogus")

	assert.Equal(t, 1, code)
	assert.Zero(t, hits.Load())
	assert.Empty(t, stdout)
	assert.Equal(t, 1, countErrorLines(stderr), stderr)
	assert.Contains(t, stderr, `invalid --severity "bogus"`)
	assert.Contains(t, stderr, "critical, high, medium, low")
}

func TestAudit_UnknownSeveritySurvivesThreshold(t *testing.T) {
	auditProject(t, `module(name = "app", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)
	// No severity data: the finding's severity cannot be determined.
	osvServer(t, http.StatusOK, `{"vulns":[{"id":"GHSA-unrated","summary":"unrated"}]}`)

	stdout, _, code := runCLI(t, "audit", "--severity=high")

	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, "GHSA-unrated")
	assert.Contains(t, stdout, "UNKNOWN")
}

func TestSeverityFiltered(t *testing.T) {
	assert.False(t, severityFiltered(osv.SeverityUnknown, osv.SeverityCritical))
	assert.False(t, severityFiltered(osv.SeverityLow, osv.SeverityUnknown))
	assert.True(t, severityFiltered(osv.SeverityLow, osv.SeverityHigh))
	assert.False(t, severityFiltered(osv.SeverityHigh, osv.SeverityHigh))
}

func TestAudit_Grammar(t *testing.T) {
	auditProject(t, `module(name = "app", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "zlib", version = "1.3.1")
`)
	osvServer(t, http.StatusOK, `{"vulns":[{"id":"GHSA-one","summary":"one"}]}`)

	stdout, stderr, code := runCLI(t, "audit", "--allow-unchecked")

	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, "Checked 1 of 2 direct dependencies.")
	assert.Contains(t, stderr, "Error: 1 vulnerability found\n")
	assert.NotContains(t, stderr, "1 vulnerabilities")

	auditProject(t, `module(name = "app", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "zlib", version = "1.3.1")
`)
	osvServer(t, http.StatusOK, `{}`)
	stdout, stderr, code = runCLI(t, "audit")
	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, "No vulnerabilities found in the 1 direct dependency that could be checked; 1 direct dependency was not checked.")
	assert.Contains(t, stderr, "1 of 2 direct dependencies was not checked")
}

func TestAudit_SeverityThresholdWording(t *testing.T) {
	proj := `module(name = "app", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	// A bare numeric CVSS score is understood by the current parser: 3.1 is LOW.
	two := `{"vulns":[{"id":"A","severity":[{"type":"CVSS_V3","score":"3.1"}]},{"id":"B","severity":[{"type":"CVSS_V3","score":"2.0"}]}]}`
	one := `{"vulns":[{"id":"A","severity":[{"type":"CVSS_V3","score":"3.1"}]}]}`

	auditProject(t, proj)
	osvServer(t, http.StatusOK, two)
	stdout, stderr, code := runCLI(t, "audit", "--severity=high")
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "No vulnerabilities at or above HIGH found")
	assert.NotContains(t, stdout, "No vulnerabilities found")
	assert.Contains(t, stdout, "2 vulnerabilities below the HIGH threshold were not shown.")

	osvServer(t, http.StatusOK, one)
	stdout, _, code = runCLI(t, "audit", "--severity=high")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "1 vulnerability below the HIGH threshold was not shown.")

	// Threshold low: the finding is shown and fails the run.
	stdout, _, code = runCLI(t, "audit", "--severity=low")
	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, "LOW")
}
