package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/osv"
)

func setupAuditTest(t *testing.T, moduleContent string) string {
	t.Helper()

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(moduleContent),
		0o644,
	))

	t.Chdir(tmpDir)

	return tmpDir
}

func resetAuditFlags() {
	auditJSON = false
	auditSeverity = ""
	auditFix = false
	auditEcosystem = ""
}

func TestAuditCmd_NoVulnerabilities(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)
	resetAuditFlags()

	// Mock client with no vulnerabilities
	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	err := auditCmd.RunE(auditCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No vulnerabilities found")
}

func TestAuditCmd_WithVulnerabilities(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-1234-5678-9abc",
					Summary:  "Critical vulnerability in rules_go",
					Severity: osv.SeverityCritical,
					Fixed:    "0.50.0",
					Link:     "https://example.com/advisory",
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	err := auditCmd.RunE(auditCmd, []string{})
	// Should return error to indicate vulnerabilities found
	require.Error(t, err)

	output := buf.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "GHSA-1234-5678-9abc")
	assert.Contains(t, output, "CRITICAL")
}

func TestAuditCmd_JSONOutput(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()
	auditJSON = true

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-test-1234",
					Summary:  "Test vulnerability",
					Severity: osv.SeverityHigh,
					Fixed:    "0.50.0",
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	_ = auditCmd.RunE(auditCmd, []string{})

	// Verify JSON output
	var result AuditResult
	err := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.Vulnerable)
	assert.Len(t, result.Vulnerabilities, 1)
	assert.Equal(t, "rules_go", result.Vulnerabilities[0].Module)
	assert.Equal(t, "GHSA-test-1234", result.Vulnerabilities[0].ID)
}

func TestAuditCmd_SeverityFilter_Critical(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()
	auditSeverity = "critical"

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-critical",
					Summary:  "Critical vulnerability",
					Severity: osv.SeverityCritical,
				},
				{
					ID:       "GHSA-high",
					Summary:  "High vulnerability",
					Severity: osv.SeverityHigh,
				},
				{
					ID:       "GHSA-low",
					Summary:  "Low vulnerability",
					Severity: osv.SeverityLow,
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	_ = auditCmd.RunE(auditCmd, []string{})

	output := buf.String()
	// Should only show critical
	assert.Contains(t, output, "GHSA-critical")
	assert.NotContains(t, output, "GHSA-high")
	assert.NotContains(t, output, "GHSA-low")
}

func TestAuditCmd_SeverityFilter_High(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()
	auditSeverity = "high"

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-critical",
					Summary:  "Critical vulnerability",
					Severity: osv.SeverityCritical,
				},
				{
					ID:       "GHSA-high",
					Summary:  "High vulnerability",
					Severity: osv.SeverityHigh,
				},
				{
					ID:       "GHSA-medium",
					Summary:  "Medium vulnerability",
					Severity: osv.SeverityMedium,
				},
				{
					ID:       "GHSA-low",
					Summary:  "Low vulnerability",
					Severity: osv.SeverityLow,
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	_ = auditCmd.RunE(auditCmd, []string{})

	output := buf.String()
	// Should show critical and high
	assert.Contains(t, output, "GHSA-critical")
	assert.Contains(t, output, "GHSA-high")
	assert.NotContains(t, output, "GHSA-medium")
	assert.NotContains(t, output, "GHSA-low")
}

func TestAuditCmd_SeverityFilter_Medium(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()
	auditSeverity = "medium"

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-high",
					Summary:  "High vulnerability",
					Severity: osv.SeverityHigh,
				},
				{
					ID:       "GHSA-medium",
					Summary:  "Medium vulnerability",
					Severity: osv.SeverityMedium,
				},
				{
					ID:       "GHSA-low",
					Summary:  "Low vulnerability",
					Severity: osv.SeverityLow,
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	_ = auditCmd.RunE(auditCmd, []string{})

	output := buf.String()
	// Should show high and medium
	assert.Contains(t, output, "GHSA-high")
	assert.Contains(t, output, "GHSA-medium")
	assert.NotContains(t, output, "GHSA-low")
}

func TestAuditCmd_SeverityFilter_Low(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()
	auditSeverity = "low"

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-low",
					Summary:  "Low vulnerability",
					Severity: osv.SeverityLow,
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	_ = auditCmd.RunE(auditCmd, []string{})

	output := buf.String()
	assert.Contains(t, output, "GHSA-low")
}

func TestAuditCmd_FixSuggestions(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()
	auditFix = true

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-1234",
					Summary:  "Vulnerability with fix",
					Severity: osv.SeverityHigh,
					Fixed:    "0.50.0",
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	_ = auditCmd.RunE(auditCmd, []string{})

	output := buf.String()
	assert.Contains(t, output, "0.50.0")
	assert.Contains(t, output, "bz mod update")
}

func TestAuditCmd_ExitCode_Vulnerabilities(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-1234",
					Severity: osv.SeverityHigh,
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	err := auditCmd.RunE(auditCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vulnerabilities found")
}

func TestAuditCmd_ExitCode_NoVulnerabilities(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)
	resetAuditFlags()

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	err := auditCmd.RunE(auditCmd, []string{})
	require.NoError(t, err)
}

func TestAuditCmd_NoDependencies(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")
`)
	resetAuditFlags()

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	err := auditCmd.RunE(auditCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No dependencies found")
}

func TestAuditCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	resetAuditFlags()

	err := auditCmd.RunE(auditCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestAuditCmd_MultipleModulesWithVulnerabilities(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
bazel_dep(name = "rules_python", version = "0.30.0")
bazel_dep(name = "rules_rust", version = "0.40.0")
`)
	resetAuditFlags()

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-go-1234",
					Summary:  "Go vulnerability",
					Severity: osv.SeverityCritical,
				},
			},
			"rules_python@0.30.0": {
				{
					ID:       "GHSA-py-5678",
					Summary:  "Python vulnerability",
					Severity: osv.SeverityMedium,
				},
			},
			// rules_rust has no vulnerabilities
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	err := auditCmd.RunE(auditCmd, []string{})
	require.Error(t, err)

	output := buf.String()
	assert.Contains(t, output, "GHSA-go-1234")
	assert.Contains(t, output, "GHSA-py-5678")
}

func TestAuditCmd_JSONOutput_FullStructure(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()
	auditJSON = true

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-1234",
					Summary:  "Test vulnerability",
					Details:  "Detailed description",
					Severity: osv.SeverityCritical,
					Aliases:  []string{"CVE-2024-1234"},
					Fixed:    "0.50.0",
					Link:     "https://example.com/advisory",
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	_ = auditCmd.RunE(auditCmd, []string{})

	var result AuditResult
	err := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.Vulnerable)
	require.Len(t, result.Vulnerabilities, 1)

	v := result.Vulnerabilities[0]
	assert.Equal(t, "rules_go", v.Module)
	assert.Equal(t, "0.46.0", v.Version)
	assert.Equal(t, "GHSA-1234", v.ID)
	assert.Equal(t, "Test vulnerability", v.Summary)
	assert.Equal(t, "CRITICAL", v.Severity)
	assert.Equal(t, "0.50.0", v.Fixed)
	assert.Equal(t, "https://example.com/advisory", v.Link)
	assert.Equal(t, []string{"CVE-2024-1234"}, v.Aliases)
}

func TestAuditCmd_APIError(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Error: context.DeadlineExceeded,
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	auditCmd.SetErr(&buf)
	t.Cleanup(func() {
		auditCmd.SetOut(os.Stdout)
		auditCmd.SetErr(os.Stderr)
	})

	err := auditCmd.RunE(auditCmd, []string{})
	// Should handle API errors gracefully
	require.Error(t, err)
}

func TestAuditCmd_UnknownSeverityIncluded(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.46.0")
`)
	resetAuditFlags()
	// No severity filter - should include unknown

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.46.0": {
				{
					ID:       "GHSA-unknown",
					Summary:  "Unknown severity",
					Severity: osv.SeverityUnknown,
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	_ = auditCmd.RunE(auditCmd, []string{})

	output := buf.String()
	assert.Contains(t, output, "GHSA-unknown")
}

func TestAuditCmd_UnknownModuleSkipped(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "unknown_module", version = "1.0.0")
`)
	resetAuditFlags()

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var outBuf, errBuf bytes.Buffer
	auditCmd.SetOut(&outBuf)
	auditCmd.SetErr(&errBuf)
	t.Cleanup(func() {
		auditCmd.SetOut(os.Stdout)
		auditCmd.SetErr(os.Stderr)
	})

	err := auditCmd.RunE(auditCmd, []string{})
	require.NoError(t, err)

	// Should show message about skipped modules
	errOutput := errBuf.String()
	assert.Contains(t, errOutput, "Skipped")
	assert.Contains(t, errOutput, "unknown_module")

	// Should show helpful message about using --ecosystem
	outOutput := outBuf.String()
	assert.Contains(t, outOutput, "No modules could be checked")
}

func TestAuditCmd_EcosystemFlag(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "unknown_module", version = "1.0.0")
`)
	resetAuditFlags()
	auditEcosystem = "Go" // Force Go ecosystem

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities:    map[string][]osv.Vulnerability{},
		SkipEcosystemCheck: true, // Skip ecosystem check since we're forcing it
	}
	t.Cleanup(func() { osvClient = oldClient })

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	err := auditCmd.RunE(auditCmd, []string{})
	require.NoError(t, err)

	// With --ecosystem flag, the unknown module should be queried
	output := buf.String()
	assert.Contains(t, output, "No vulnerabilities found")
}

func TestAuditCmd_MixedKnownUnknownModules(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.0")
bazel_dep(name = "unknown_module", version = "1.0.0")
`)
	resetAuditFlags()

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{
			"rules_go@0.50.0": {
				{
					ID:       "GHSA-test",
					Summary:  "Test vulnerability",
					Severity: osv.SeverityHigh,
				},
			},
		},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var outBuf, errBuf bytes.Buffer
	auditCmd.SetOut(&outBuf)
	auditCmd.SetErr(&errBuf)
	t.Cleanup(func() {
		auditCmd.SetOut(os.Stdout)
		auditCmd.SetErr(os.Stderr)
	})

	err := auditCmd.RunE(auditCmd, []string{})
	// Should return error because vulnerability found
	require.Error(t, err)

	// Should report skipped module
	errOutput := errBuf.String()
	assert.Contains(t, errOutput, "Skipped")
	assert.Contains(t, errOutput, "unknown_module")

	// Should still report vulnerability for known module
	outOutput := outBuf.String()
	assert.Contains(t, outOutput, "GHSA-test")
}

func TestAuditCmd_KnownModuleNoOSVEquivalent(t *testing.T) {
	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_cc", version = "0.0.9")
`)
	resetAuditFlags()

	oldClient := osvClient
	osvClient = &osv.MockClient{
		Vulnerabilities: map[string][]osv.Vulnerability{},
	}
	t.Cleanup(func() { osvClient = oldClient })

	var outBuf, errBuf bytes.Buffer
	auditCmd.SetOut(&outBuf)
	auditCmd.SetErr(&errBuf)
	t.Cleanup(func() {
		auditCmd.SetOut(os.Stdout)
		auditCmd.SetErr(os.Stderr)
	})

	err := auditCmd.RunE(auditCmd, []string{})
	require.NoError(t, err)

	// Should show message about module with no OSV equivalent
	errOutput := errBuf.String()
	assert.Contains(t, errOutput, "rules_cc")
	assert.Contains(t, errOutput, "no OSV equivalent")
}

func TestAuditCmd_OfflineMode(t *testing.T) {
	cli.ResetCachedConfig()
	t.Cleanup(cli.ResetCachedConfig)

	setupAuditTest(t, `module(name = "test", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)
	resetAuditFlags()

	// Set offline mode via environment variable
	t.Setenv("BZ_OFFLINE", "1")

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	err := auditCmd.RunE(auditCmd, []string{})
	require.Error(t, err)

	// Error message should indicate offline mode issue
	errMsg := err.Error()
	assert.Contains(t, errMsg, "audit")
	assert.Contains(t, errMsg, "network")
	assert.Contains(t, errMsg, "offline")
}

func TestAuditCmd_DisabledViaConfig(t *testing.T) {
	cli.ResetCachedConfig()
	t.Cleanup(cli.ResetCachedConfig)

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
`),
		0o644,
	))

	// Create config that disables audit
	configContent := `
[commands]
disabled = ["audit"]
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".bzconfig.toml"), []byte(configContent), 0o644))

	resetAuditFlags()

	var buf bytes.Buffer
	auditCmd.SetOut(&buf)
	t.Cleanup(func() { auditCmd.SetOut(os.Stdout) })

	err := auditCmd.RunE(auditCmd, []string{})
	require.Error(t, err)

	// Error message should indicate command is disabled
	errMsg := err.Error()
	assert.Contains(t, errMsg, "audit")
	assert.Contains(t, errMsg, "disabled")
}
