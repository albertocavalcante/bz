package cli

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
	assert.False(t, opts.Offline, "Offline should be false by default")
	assert.False(t, opts.PreferOffline, "PreferOffline should be false by default")
	assert.Empty(t, opts.Registry, "Registry should be empty by default")
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

func TestConfigureLipgloss(t *testing.T) {
	tests := []struct {
		name           string
		noColor        bool
		envSet         bool
		expectNoStyles bool
	}{
		{
			name:           "colors enabled - styles should render",
			noColor:        false,
			envSet:         false,
			expectNoStyles: false,
		},
		{
			name:           "flag disables colors - styles should not render",
			noColor:        true,
			envSet:         false,
			expectNoStyles: true,
		},
		{
			name:           "env disables colors - styles should not render",
			noColor:        false,
			envSet:         true,
			expectNoStyles: true,
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

			// Call ConfigureLipgloss - this should not panic
			ConfigureLipgloss()

			// Test that a styled string behaves correctly
			style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
			styledText := style.Render("test")

			if tt.expectNoStyles {
				// When colors are disabled, the rendered text should be plain "test"
				assert.Equal(t, "test", styledText, "styled text should be plain when colors disabled")
			} else {
				// When colors are enabled, we can't guarantee ANSI codes in CI environment
				// but we can verify the function doesn't panic and returns something
				assert.NotEmpty(t, styledText, "styled text should not be empty")
			}
		})
	}
}

func TestIsOffline(t *testing.T) {
	tests := []struct {
		name     string
		offline  bool
		envVal   string
		envSet   bool
		expected bool
	}{
		{
			name:     "offline disabled by default",
			offline:  false,
			envSet:   false,
			expected: false,
		},
		{
			name:     "offline enabled via flag",
			offline:  true,
			envSet:   false,
			expected: true,
		},
		{
			name:     "offline enabled via BZ_OFFLINE env var",
			offline:  false,
			envVal:   "1",
			envSet:   true,
			expected: true,
		},
		{
			name:     "offline enabled via BZ_OFFLINE=true",
			offline:  false,
			envVal:   "true",
			envSet:   true,
			expected: true,
		},
		{
			name:     "offline disabled via BZ_OFFLINE=0",
			offline:  false,
			envVal:   "0",
			envSet:   true,
			expected: false,
		},
		{
			name:     "offline disabled via BZ_OFFLINE=false",
			offline:  false,
			envVal:   "false",
			envSet:   true,
			expected: false,
		},
		{
			name:     "offline disabled via empty BZ_OFFLINE",
			offline:  false,
			envVal:   "",
			envSet:   true,
			expected: false,
		},
		{
			name:     "flag takes precedence over env",
			offline:  true,
			envVal:   "0",
			envSet:   true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Global = Options{Offline: tt.offline}

			oldVal, hadVal := os.LookupEnv("BZ_OFFLINE")
			if tt.envSet {
				os.Setenv("BZ_OFFLINE", tt.envVal)
			} else {
				os.Unsetenv("BZ_OFFLINE")
			}
			defer func() {
				if hadVal {
					os.Setenv("BZ_OFFLINE", oldVal)
				} else {
					os.Unsetenv("BZ_OFFLINE")
				}
			}()

			result := IsOffline()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPreferOffline(t *testing.T) {
	tests := []struct {
		name          string
		preferOffline bool
		envVal        string
		envSet        bool
		expected      bool
	}{
		{
			name:          "prefer-offline disabled by default",
			preferOffline: false,
			envSet:        false,
			expected:      false,
		},
		{
			name:          "prefer-offline enabled via flag",
			preferOffline: true,
			envSet:        false,
			expected:      true,
		},
		{
			name:          "prefer-offline enabled via BZ_PREFER_OFFLINE env var",
			preferOffline: false,
			envVal:        "1",
			envSet:        true,
			expected:      true,
		},
		{
			name:          "prefer-offline disabled via BZ_PREFER_OFFLINE=0",
			preferOffline: false,
			envVal:        "0",
			envSet:        true,
			expected:      false,
		},
		{
			name:          "flag takes precedence over env",
			preferOffline: true,
			envVal:        "0",
			envSet:        true,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Global = Options{PreferOffline: tt.preferOffline}

			oldVal, hadVal := os.LookupEnv("BZ_PREFER_OFFLINE")
			if tt.envSet {
				os.Setenv("BZ_PREFER_OFFLINE", tt.envVal)
			} else {
				os.Unsetenv("BZ_PREFER_OFFLINE")
			}
			defer func() {
				if hadVal {
					os.Setenv("BZ_PREFER_OFFLINE", oldVal)
				} else {
					os.Unsetenv("BZ_PREFER_OFFLINE")
				}
			}()

			result := IsPreferOffline()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRegistry(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		envVal   string
		envSet   bool
		expected string
	}{
		{
			name:     "empty by default",
			registry: "",
			envSet:   false,
			expected: "",
		},
		{
			name:     "registry set via flag",
			registry: "https://custom.registry.io",
			envSet:   false,
			expected: "https://custom.registry.io",
		},
		{
			name:     "registry set via BZ_REGISTRY env var",
			registry: "",
			envVal:   "https://env.registry.io",
			envSet:   true,
			expected: "https://env.registry.io",
		},
		{
			name:     "flag takes precedence over env",
			registry: "https://flag.registry.io",
			envVal:   "https://env.registry.io",
			envSet:   true,
			expected: "https://flag.registry.io",
		},
		{
			name:     "empty env is ignored",
			registry: "",
			envVal:   "",
			envSet:   true,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Global = Options{Registry: tt.registry}

			oldVal, hadVal := os.LookupEnv("BZ_REGISTRY")
			if tt.envSet {
				os.Setenv("BZ_REGISTRY", tt.envVal)
			} else {
				os.Unsetenv("BZ_REGISTRY")
			}
			defer func() {
				if hadVal {
					os.Setenv("BZ_REGISTRY", oldVal)
				} else {
					os.Unsetenv("BZ_REGISTRY")
				}
			}()

			result := GetRegistry()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateOfflineFlags(t *testing.T) {
	tests := []struct {
		name          string
		offline       bool
		preferOffline bool
		expectError   bool
	}{
		{
			name:          "both false - no error",
			offline:       false,
			preferOffline: false,
			expectError:   false,
		},
		{
			name:          "only offline - no error",
			offline:       true,
			preferOffline: false,
			expectError:   false,
		},
		{
			name:          "only prefer-offline - no error",
			offline:       false,
			preferOffline: true,
			expectError:   false,
		},
		{
			name:          "both set - error",
			offline:       true,
			preferOffline: true,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Global = Options{
				Offline:       tt.offline,
				PreferOffline: tt.preferOffline,
			}

			err := ValidateOfflineFlags()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "mutually exclusive")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
