// Package cli provides global CLI options and utilities.
package cli

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Options holds global CLI options that affect output behavior.
type Options struct {
	// Quiet reduces output to only errors and essential information.
	Quiet bool

	// NoColor disables colored output.
	NoColor bool
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
