// Package cli provides global CLI options and utilities.
package cli

import (
	"github.com/charmbracelet/lipgloss"
)

// Style variables for consistent coloring across the CLI.
// These use ANSI 256 color codes for broad terminal compatibility.
var (
	// StyleSuccess is used for success messages (green).
	StyleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	// StyleWarning is used for warning messages (yellow).
	StyleWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	// StyleError is used for error messages (red).
	StyleError = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	// StyleInfo is used for informational messages (blue).
	StyleInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))

	// StyleMuted is used for less important text (gray).
	StyleMuted = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// StyleBold is used for emphasized text.
	StyleBold = lipgloss.NewStyle().Bold(true)
)

// Success returns a green-colored string if colors are enabled.
func Success(s string) string {
	return StyleSuccess.Render(s)
}

// Warning returns a yellow-colored string if colors are enabled.
func Warning(s string) string {
	return StyleWarning.Render(s)
}

// Error returns a red-colored string if colors are enabled.
func Error(s string) string {
	return StyleError.Render(s)
}

// Info returns a blue-colored string if colors are enabled.
func Info(s string) string {
	return StyleInfo.Render(s)
}

// Muted returns a gray-colored string if colors are enabled.
func Muted(s string) string {
	return StyleMuted.Render(s)
}

// Bold returns a bold string if styling is enabled.
func Bold(s string) string {
	return StyleBold.Render(s)
}

// SuccessIcon returns a green checkmark.
func SuccessIcon() string {
	return StyleSuccess.Render("\u2713")
}

// ErrorIcon returns a red X mark.
func ErrorIcon() string {
	return StyleError.Render("\u2717")
}

// WarningIcon returns a yellow warning symbol.
func WarningIcon() string {
	return StyleWarning.Render("!")
}

// InfoIcon returns a blue info symbol.
func InfoIcon() string {
	return StyleInfo.Render("i")
}
