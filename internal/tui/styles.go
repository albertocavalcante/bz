package tui

import "github.com/charmbracelet/lipgloss"

// Theme colors
var (
	Primary   = lipgloss.Color("99")  // Purple
	Secondary = lipgloss.Color("39")  // Cyan
	Success   = lipgloss.Color("78")  // Green
	Warning   = lipgloss.Color("220") // Yellow
	Error     = lipgloss.Color("196") // Red
	Muted     = lipgloss.Color("241") // Gray
)

// Styles defines all application styles
type Styles struct {
	App       lipgloss.Style
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Selected  lipgloss.Style
	Normal    lipgloss.Style
	Muted     lipgloss.Style
	Success   lipgloss.Style
	Error     lipgloss.Style
	Help      lipgloss.Style
	StatusBar lipgloss.Style
}

// DefaultStyles returns the default style configuration
func DefaultStyles() Styles {
	return Styles{
		App: lipgloss.NewStyle().
			Padding(1, 2),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary),

		Subtitle: lipgloss.NewStyle().
			Foreground(Secondary),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(Primary).
			Padding(0, 1),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),

		Muted: lipgloss.NewStyle().
			Foreground(Muted),

		Success: lipgloss.NewStyle().
			Foreground(Success),

		Error: lipgloss.NewStyle().
			Foreground(Error),

		Help: lipgloss.NewStyle().
			Foreground(Muted).
			MarginTop(1),

		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("236")).
			Padding(0, 1),
	}
}
