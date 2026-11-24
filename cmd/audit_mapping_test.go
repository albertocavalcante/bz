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
	const reason = "no package mapping: OSV has no records under this module name"
	const help = "help: bz has no module-to-upstream-package mapping yet; --ecosystem only finds modules named exactly like their upstream package\n"
	text := "Checked 0 of 3 direct dependencies.\n" +
		"Not checked (3):\n" +
		"  - rules_go@0.50.1: " + reason + "\n" +
		"  - rules_python@0.34.0: " + reason + "\n" +
		"  - gazelle@0.38.0: " + reason + "\n" +
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

func TestAudit_ForcedEcosystemStatesItsAssumption(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
	}{
		{"text", []string{"audit", "--ecosystem=Go"},
			"Checked 3 of 3 direct dependencies as Go packages named after the module.\n" +
				"No vulnerabilities found\n"},
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
			assert.Empty(t, stderr)
			assert.Equal(t, 0, code)
			assert.EqualValues(t, 3, hits.Load())
		})
	}
}
