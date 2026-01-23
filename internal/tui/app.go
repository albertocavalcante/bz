package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// State represents the current application state (state machine)
type State int

const (
	StateLoading State = iota
	StateList
	StateInfo
	StateSearch
	StateError
)

// App is the root model following Elm architecture
type App struct {
	// Current state
	state State

	// Sub-models (composed, not inherited)
	list    ListModel
	spinner spinner.Model

	// Application state
	styles    Styles
	width     int
	height    int
	err       error
	statusMsg string
}

// NewApp creates a new application model
func NewApp() App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Primary)

	return App{
		state:   StateLoading,
		spinner: s,
		styles:  DefaultStyles(),
	}
}

// Init implements tea.Model - returns initial command
func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		ListLocalDeps(),
	)
}

// Update implements tea.Model - handles all messages
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// Window resize
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.list, _ = a.list.Update(msg)
		return a, nil

	// Keyboard input
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		}

	// Error handling
	case ErrMsg:
		a.state = StateError
		a.err = msg.Err
		return a, nil

	// Dependencies listed
	case DepsListedMsg:
		items := make([]ModuleItem, len(msg.Module.Dependencies))
		for i, dep := range msg.Module.Dependencies {
			items[i] = ModuleItem{
				name:    dep.Name,
				version: dep.Version,
				dev:     dep.DevDependency,
			}
		}

		title := "Dependencies"
		if msg.Module.Name != "" {
			title = fmt.Sprintf("%s - Dependencies", msg.Module.Name)
		}

		a.list = NewListModel(title, items, a.styles)
		a.state = StateList
		return a, nil

	// Module info fetched
	case ModuleInfoMsg:
		a.state = StateInfo
		// Could switch to an info view here
		return a, nil

	// Spinner tick
	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Delegate to sub-models based on state
	switch a.state {
	case StateList:
		var cmd tea.Cmd
		a.list, cmd = a.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// View implements tea.Model - renders the current view
func (a App) View() string {
	switch a.state {
	case StateLoading:
		return a.viewLoading()
	case StateList:
		return a.list.View()
	case StateError:
		return a.viewError()
	default:
		return ""
	}
}

func (a App) viewLoading() string {
	return a.styles.App.Render(
		fmt.Sprintf("%s Loading...", a.spinner.View()),
	)
}

func (a App) viewError() string {
	return a.styles.App.Render(
		a.styles.Error.Render(fmt.Sprintf("Error: %v", a.err)) +
			"\n\n" +
			a.styles.Help.Render("Press q to quit"),
	)
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(
		NewApp(),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}

// RunHeadless runs in headless mode (no TUI)
func RunHeadless() error {
	// For headless mode, directly execute commands and print output
	cmd := ListLocalDeps()
	msg := cmd()

	styles := DefaultStyles()

	switch m := msg.(type) {
	case ErrMsg:
		return m.Err
	case DepsListedMsg:
		items := make([]ModuleItem, len(m.Module.Dependencies))
		for i, dep := range m.Module.Dependencies {
			items[i] = ModuleItem{
				name:    dep.Name,
				version: dep.Version,
				dev:     dep.DevDependency,
			}
		}

		if m.Module.Name != "" {
			fmt.Printf("%s", styles.Title.Render(m.Module.Name))
			if m.Module.Version != "" {
				fmt.Printf(" %s", styles.Muted.Render("("+m.Module.Version+")"))
			}
			fmt.Println()
		}

		fmt.Println(RenderDepsTable(items, styles))
	}

	return nil
}
