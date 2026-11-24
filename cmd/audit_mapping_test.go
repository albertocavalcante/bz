package cmd

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const rulesetsOnly = `module(name = "app", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.34.0")
bazel_dep(name = "gazelle", version = "0.38.0")
`

// These modules used to be sent to OSV under their module names, which match
// no OSV package, and then counted as checked.
func TestAudit_ModuleNameIsNotAPackageName(t *testing.T) {
	const reason = "no package mapping: bz does not know which upstream package this module corresponds to"
	const help = "help: vulnerability matching for Bazel modules is not available yet; --allow-unchecked accepts that and audits nothing\n"
	text := "Checked 0 of 3 direct dependencies.\n" +
		"Not checked (3): " + reason + "\n" +
		"  - rules_go@0.50.1\n" +
		"  - rules_python@0.34.0\n" +
		"  - gazelle@0.38.0\n" +
		help +
		"No direct dependency could be checked; no vulnerability data was obtained.\n"
	json := `{
  "total": 3,
  "vulnerable": 0,
  "vulnerabilities": [],
  "summary": {
    "scope": "direct",
    "total": 3,
    "checked": 0,
    "unchecked": 3,
    "failed": 0
  },
  "unchecked": [
    {
      "module": "rules_go",
      "version": "0.50.1",
      "reason": "` + reason + `"
    },
    {
      "module": "rules_python",
      "version": "0.34.0",
      "reason": "` + reason + `"
    },
    {
      "module": "gazelle",
      "version": "0.38.0",
      "reason": "` + reason + `"
    }
  ]
}
`
	failure := "Error: 3 of 3 direct dependencies were not checked (use --allow-unchecked to accept this)\n"

	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantStderr string
		wantCode   int
	}{
		{"text", []string{"audit"}, text, failure, 1},
		{"json", []string{"audit", "--json"}, json, failure, 1},
		{"text allow-unchecked", []string{"audit", "--allow-unchecked"}, text, "", 0},
		{"json allow-unchecked", []string{"audit", "--json", "--allow-unchecked"}, json, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditProject(t, rulesetsOnly)
			hits := osvServer(t, http.StatusOK, `{}`)

			stdout, stderr, code := runCLI(t, tt.args...)

			assert.Equal(t, tt.wantStdout, stdout)
			assert.Equal(t, tt.wantStderr, stderr)
			assert.Equal(t, tt.wantCode, code)
			assert.Zero(t, hits.Load(), "no query can match, so none may be sent")
		})
	}
}

const forcedNote = "note: --ecosystem=Go queries module names as package names; exit status 0 does not mean the modules were audited\n"

func TestAudit_ForcedEcosystemNeverClaimsClean(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
	}{
		{"text", []string{"audit", "--ecosystem=Go"},
			"Queried 3 of 3 direct dependencies as Go packages named after the module.\n" +
				"No OSV records found under these names in Go; this does not show the modules are free of vulnerabilities.\n"},
		{"lowercase is normalised", []string{"audit", "--ecosystem=go"},
			"Queried 3 of 3 direct dependencies as Go packages named after the module.\n" +
				"No OSV records found under these names in Go; this does not show the modules are free of vulnerabilities.\n"},
		{"threshold", []string{"audit", "--ecosystem=Go", "--severity=high"},
			"Queried 3 of 3 direct dependencies as Go packages named after the module.\n" +
				"No OSV records at or above HIGH found under these names in Go; this does not show the modules are free of vulnerabilities.\n"},
		{"json", []string{"audit", "--json", "--ecosystem=Go"}, `{
  "total": 3,
  "vulnerable": 0,
  "vulnerabilities": [],
  "summary": {
    "scope": "direct",
    "ecosystem": "Go",
    "total": 3,
    "checked": 3,
    "unchecked": 0,
    "failed": 0
  },
  "unchecked": []
}
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditProject(t, rulesetsOnly)
			hits := osvServer(t, http.StatusOK, `{}`)

			stdout, stderr, code := runCLI(t, tt.args...)

			assert.Equal(t, tt.wantStdout, stdout)
			assert.Equal(t, forcedNote, stderr)
			assert.Equal(t, 0, code)
			assert.EqualValues(t, 3, hits.Load())
		})
	}
}

func TestAudit_ForcedEcosystemSingular(t *testing.T) {
	auditProject(t, `module(name = "app", version = "1.0.0")

bazel_dep(name = "protobuf", version = "27.1")
`)
	osvServer(t, http.StatusOK, `{}`)

	stdout, _, code := runCLI(t, "audit", "--ecosystem=PyPI")

	assert.Equal(t, 0, code)
	assert.Equal(t, "Queried 1 of 1 direct dependency as a PyPI package named after the module.\n"+
		"No OSV records found under these names in PyPI; this does not show the modules are free of vulnerabilities.\n", stdout)
}

func TestAudit_ForcedEcosystemWithFindings(t *testing.T) {
	const fixture = `{"vulns":[{"id":"GHSA-0000-0000-0000","summary":"Example advisory from a test fixture","severity":[{"type":"CVSS_V3","score":"8.1"}]}]}`
	const want = "Module    Version  ID                   Severity  Summary\n" +
		"------    -------  --                   --------  -------\n" +
		"rules_go  0.50.1   GHSA-0000-0000-0000  HIGH      Example advisory from a test fixture\n" +
		"\n" +
		"Queried 1 of 1 direct dependency as a Go package named after the module.\n"
	const wantErr = "Error: 1 vulnerability found\n"

	for _, args := range [][]string{
		{"audit", "--ecosystem=Go"},
		{"audit", "--ecosystem=Go", "--allow-unchecked"},
	} {
		auditProject(t, `module(name = "app", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)
		osvServer(t, http.StatusOK, fixture)

		stdout, stderr, code := runCLI(t, args...)

		assert.Equal(t, want, stdout, args)
		assert.Equal(t, wantErr, stderr, args)
		assert.Equal(t, 1, code, args)
	}
}

func TestAudit_EcosystemValidation(t *testing.T) {
	const list = "https://ossf.github.io/osv-schema/#affectedpackage-field"
	tests := []struct {
		name string
		flag string
		want string
	}{
		{"typo", "--ecosystem=pypy", `Error: invalid --ecosystem: unknown OSV ecosystem "pypy": did you mean "PyPI"? (valid names: ` + list + ")\n"},
		{"near miss in case", "--ecosystem=Mavn", `Error: invalid --ecosystem: unknown OSV ecosystem "Mavn": did you mean "Maven"? (valid names: ` + list + ")\n"},
		{"unknown", "--ecosystem=bazel", `Error: invalid --ecosystem: unknown OSV ecosystem "bazel" (valid names: ` + list + ")\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditProject(t, rulesetsOnly)
			hits := osvServer(t, http.StatusOK, `{}`)

			stdout, stderr, code := runCLI(t, "audit", tt.flag)

			assert.Empty(t, stdout)
			assert.Equal(t, tt.want, stderr)
			assert.Equal(t, 1, code)
			assert.Zero(t, hits.Load())
		})
	}
}
