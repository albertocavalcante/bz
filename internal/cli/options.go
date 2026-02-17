// Package cli provides global CLI options and utilities.
package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/albertocavalcante/bz/internal/envutil"
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
// This is set by the root command's PersistentPreRun.
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

// IsOffline returns true if strict offline mode is enabled.
// This also respects the BZ_OFFLINE environment variable.
func IsOffline() bool {
	if Global.Offline {
		return true
	}
	return envutil.IsTruthyEnv("BZ_OFFLINE")
}

// IsPreferOffline returns true if cache-first mode is enabled.
// This also respects the BZ_PREFER_OFFLINE environment variable.
func IsPreferOffline() bool {
	if Global.PreferOffline {
		return true
	}
	return envutil.IsTruthyEnv("BZ_PREFER_OFFLINE")
}

// GetRegistry returns the registry URL if overridden, empty string otherwise.
// This also respects the BZ_REGISTRY environment variable.
func GetRegistry() string {
	if Global.Registry != "" {
		return Global.Registry
	}
	// Also respect BZ_REGISTRY environment variable
	if val, ok := os.LookupEnv("BZ_REGISTRY"); ok && val != "" {
		return val
	}
	return ""
}

// ValidateOfflineFlags checks that --offline and --prefer-offline are not both set.
// Returns an error if both flags are set simultaneously.
func ValidateOfflineFlags() error {
	if Global.Offline && Global.PreferOffline {
		return fmt.Errorf("--offline and --prefer-offline are mutually exclusive")
	}
	return nil
}
