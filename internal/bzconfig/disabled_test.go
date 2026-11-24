package bzconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCommands = []string{
	"audit", "cache", "cache clear", "cache download", "cache stats", "cache verify",
	"completion", "completion bash", "doctor", "help", "init",
	"mod", "mod add", "mod info", "mod stats", "registry", "registry ping", "version",
}

var testSource = Source{Layer: LayerProject, Origin: "/ws/.bz.toml"}

func TestResolveDisabled(t *testing.T) {
	tests := []struct {
		name         string
		entries      []string
		disabled     []string // paths that must match
		enabled      []string // paths that must not match
		wantWarnings []string
		wantErr      []string
	}{
		{
			name: "full path", entries: []string{"registry ping"},
			disabled: []string{"registry ping"}, enabled: []string{"registry", "mod info"},
		},
		{
			name: "bare alias", entries: []string{"ping", "add"},
			disabled: []string{"registry ping", "mod add"}, enabled: []string{"mod info"},
		},
		{
			name: "top-level command by name", entries: []string{"audit"},
			disabled: []string{"audit"}, enabled: []string{"doctor"},
		},
		{
			name: "parent disables children", entries: []string{"cache"},
			disabled: []string{"cache", "cache download", "cache stats"}, enabled: []string{"mod stats"},
		},
		{
			name: "extra spaces and bz prefix", entries: []string{"  bz   registry  ping "},
			disabled: []string{"registry ping"},
		},
		{
			name: "full path disambiguates", entries: []string{"cache stats"},
			disabled: []string{"cache stats"}, enabled: []string{"mod stats"},
		},
		{name: "ambiguous bare name", entries: []string{"stats"}, wantErr: []string{`"stats"`, `"cache stats"`, `"mod stats"`, testSource.String()}},
		{
			name: "unknown entries warn", entries: []string{"nosuch", "registry pong"},
			wantWarnings: []string{`"nosuch"`, `"registry pong"`, "matches no command"},
		},
		{
			name: "protected commands cannot be disabled", entries: []string{"help", "version", "completion bash"},
			enabled:      []string{"help", "version", "completion bash"},
			wantWarnings: []string{"cannot be disabled"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, warnings, err := ResolveDisabled(tt.entries, testCommands, testSource)
			if len(tt.wantErr) > 0 {
				require.Error(t, err)
				for _, sub := range tt.wantErr {
					assert.Contains(t, err.Error(), sub)
				}
				return
			}
			require.NoError(t, err)
			for _, p := range tt.disabled {
				_, ok := set.Match(p)
				assert.True(t, ok, "%q should be disabled", p)
			}
			for _, p := range tt.enabled {
				_, ok := set.Match(p)
				assert.False(t, ok, "%q should be enabled", p)
			}
			joined := strings.Join(warnings, "\n")
			if len(tt.wantWarnings) == 0 {
				assert.Empty(t, warnings)
			}
			for _, sub := range tt.wantWarnings {
				assert.Contains(t, joined, sub)
			}
		})
	}
}

func TestDisabledSetMatchReturnsEntry(t *testing.T) {
	set, _, err := ResolveDisabled([]string{"ping"}, testCommands, testSource)
	require.NoError(t, err)
	entry, ok := set.Match("registry ping")
	assert.True(t, ok)
	assert.Equal(t, "ping", entry)
	assert.Equal(t, 1, set.Len())
}

func TestIsProtectedCommand(t *testing.T) {
	for _, p := range []string{"", "help", "version", "completion", "completion zsh", "__complete"} {
		assert.True(t, IsProtectedCommand(p), p)
	}
	for _, p := range []string{"mod", "registry ping", "doctor"} {
		assert.False(t, IsProtectedCommand(p), p)
	}
}
