package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun_DisabledCommands(t *testing.T) {
	tests := []struct {
		name     string
		disabled string
		args     []string
		wantCode int
		// wantErr must appear in the single Error: line when wantCode != 0.
		wantErr []string
		// wantWarn must appear on a Warning: line.
		wantWarn []string
	}{
		{
			name: "full path", disabled: `["registry ping"]`,
			args: []string{"registry", "ping", "http://127.0.0.1:1"}, wantCode: 1,
			wantErr: []string{`command "registry ping" is disabled`, `listed as "registry ping"`, ".bz.toml"},
		},
		{
			name: "bare alias", disabled: `["ping"]`,
			args: []string{"registry", "ping", "http://127.0.0.1:1"}, wantCode: 1,
			wantErr: []string{`command "registry ping" is disabled`, `listed as "ping"`},
		},
		{
			name: "command that used to check nothing", disabled: `["cache stats"]`,
			args: []string{"cache", "stats"}, wantCode: 1,
			wantErr: []string{`command "cache stats" is disabled`},
		},
		{
			name: "parent disables child", disabled: `["cache"]`,
			args: []string{"cache", "verify"}, wantCode: 1,
			wantErr: []string{`command "cache verify" is disabled`, `listed as "cache"`},
		},
		{
			name: "ambiguous alias", disabled: `["stats"]`,
			args: []string{"mod", "list"}, wantCode: 1,
			wantErr: []string{"ambiguous", `"cache stats"`, `"mod stats"`},
		},
		{
			name: "unknown entry warns", disabled: `["nosuch", "registry pong"]`,
			args: []string{"version"}, wantCode: 0,
			wantWarn: []string{`"nosuch"`, `"registry pong"`, "matches no command"},
		},
		{
			name: "version cannot be disabled", disabled: `["version"]`,
			args: []string{"version"}, wantCode: 0,
			wantWarn: []string{"cannot be disabled"},
		},
		{
			name: "help survives an ambiguous entry", disabled: `["stats"]`,
			args: []string{"help"}, wantCode: 0,
			wantWarn: []string{"ambiguous"},
		},
		{
			name: "other commands stay enabled", disabled: `["registry ping"]`,
			args: []string{"cache", "stats", "--cache-dir", "CACHE"}, wantCode: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws, _ := configWorkspace(t, "[commands]\ndisabled = "+tt.disabled+"\n", "")
			args := make([]string, len(tt.args))
			for i, a := range tt.args {
				args[i] = strings.ReplaceAll(a, "CACHE", filepath.Join(ws, "cache"))
			}

			_, stderr, code := runCLI(t, args...)
			assert.Equal(t, tt.wantCode, code, "stderr: %s", stderr)
			if tt.wantCode != 0 {
				assert.Equal(t, 1, countErrorLines(stderr), "stderr: %s", stderr)
			} else {
				assert.Zero(t, countErrorLines(stderr), "stderr: %s", stderr)
			}
			for _, sub := range tt.wantErr {
				assert.Contains(t, stderr, sub)
			}
			for _, sub := range tt.wantWarn {
				assert.Contains(t, stderr, "Warning:")
				assert.Contains(t, stderr, sub)
			}
		})
	}
}

func TestRun_DisabledViaEnvOverridesFile(t *testing.T) {
	configWorkspace(t, "[commands]\ndisabled = [\"registry ping\"]\n", "")
	t.Setenv("BZ_DISABLE_COMMANDS", "audit")

	_, stderr, code := runCLI(t, "audit")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "environment BZ_DISABLE_COMMANDS")
	assert.Equal(t, 1, countErrorLines(stderr))
}
