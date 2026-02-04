package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsColorEnabled_FlagTakesPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		noColor  bool
		envSet   bool
		expected bool
	}{
		{
			name:     "color enabled by default",
			noColor:  false,
			envSet:   false,
			expected: true,
		},
		{
			name:     "flag disables color",
			noColor:  true,
			envSet:   false,
			expected: false,
		},
		{
			name:     "env var disables color",
			noColor:  false,
			envSet:   true,
			expected: false,
		},
		{
			name:     "both flag and env disable color",
			noColor:  true,
			envSet:   true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global state
			Global = Options{NoColor: tt.noColor}

			// Handle NO_COLOR env var
			oldVal, hadVal := os.LookupEnv("NO_COLOR")
			if tt.envSet {
				os.Setenv("NO_COLOR", "1")
			} else {
				os.Unsetenv("NO_COLOR")
			}
			defer func() {
				if hadVal {
					os.Setenv("NO_COLOR", oldVal)
				} else {
					os.Unsetenv("NO_COLOR")
				}
			}()

			result := IsColorEnabled()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOptions_DefaultValues(t *testing.T) {
	opts := Options{}
	assert.False(t, opts.Quiet, "Quiet should be false by default")
	assert.False(t, opts.NoColor, "NoColor should be false by default")
}

func TestIsQuiet(t *testing.T) {
	tests := []struct {
		name     string
		quiet    bool
		expected bool
	}{
		{
			name:     "quiet disabled by default",
			quiet:    false,
			expected: false,
		},
		{
			name:     "quiet enabled",
			quiet:    true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Global = Options{Quiet: tt.quiet}
			result := IsQuiet()
			assert.Equal(t, tt.expected, result)
		})
	}
}
