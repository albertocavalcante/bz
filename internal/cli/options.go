// Package cli provides global CLI options and utilities.
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/albertocavalcante/bz/internal/bzconfig"
)

// Options holds global CLI options that affect output behavior.
type Options struct {
	// Quiet reduces output to only errors and essential information.
	Quiet bool

	// NoColor disables colored output.
	NoColor bool

	// Offline enables strict offline mode - no network access, use cache only.
	// Useful for air-gapped environments.
	Offline bool

	// PreferOffline prefers cached data but falls back to network if needed.
	// This is a cache-first strategy.
	PreferOffline bool

	// Registry overrides the default registry URL.
	Registry string
}

// Global holds the current CLI options.
// It is populated by the root command's persistent flags.
var Global Options

// IsQuiet returns true if quiet mode is enabled.
// Use this to skip non-essential informational output.
func IsQuiet() bool {
	return Global.Quiet
}

// IsColorEnabled returns true if colored output should be used.
// It checks both the --no-color flag and environment variables.
func IsColorEnabled() bool {
	if Global.NoColor {
		return false
	}
	// Also respect NO_COLOR environment variable (https://no-color.org/)
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return true
}

// ConfigureLipgloss sets up lipgloss based on color settings.
// Call this after parsing flags to ensure colors are properly disabled.
func ConfigureLipgloss() {
	if !IsColorEnabled() {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// IsOffline returns true if the effective network mode is offline
// (from --offline, BZ_OFFLINE or [network] mode in a config file).
func IsOffline() bool {
	return effectiveOrFlags().IsOffline()
}

// IsPreferOffline returns true if the effective network mode is
// prefer-offline (from --prefer-offline, BZ_PREFER_OFFLINE or config).
func IsPreferOffline() bool {
	return effectiveOrFlags().IsPreferOffline()
}

// GetRegistry returns the effective registry URL when any layer (config
// file, BZ_REGISTRY or --registry) sets one, and "" when the built-in
// default applies, so callers can choose their own default client.
func GetRegistry() string {
	eff := effectiveOrFlags()
	if eff.Source(bzconfig.KeyRegistry).Layer == bzconfig.LayerDefault {
		return ""
	}
	return eff.Network.Registry
}

// FallbackRegistries returns the effective [network] fallback_registries.
func FallbackRegistries() []string {
	return effectiveOrFlags().Network.FallbackRegistries
}

// Timeout returns the effective [network] timeout.
func Timeout() time.Duration {
	return effectiveOrFlags().Network.TimeoutDuration
}

// CacheDir returns the effective cache directory (BZ_CACHE_DIR, then
// [cache] dir, then the platform default). Per-command --cache-dir flags
// take precedence and are applied by the commands that define them.
// It fails when the configuration cannot be loaded.
func CacheDir() (string, error) {
	eff, err := Effective()
	if err != nil {
		return "", err
	}
	return eff.Cache.Dir, nil
}

// ValidateOfflineFlags checks that --offline and --prefer-offline are not both set.
// Returns an error if both flags are set simultaneously.
func ValidateOfflineFlags() error {
	if Global.Offline && Global.PreferOffline {
		return fmt.Errorf("--offline and --prefer-offline are mutually exclusive")
	}
	return nil
}
