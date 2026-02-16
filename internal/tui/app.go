package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/albertocavalcante/bz/internal/module"
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
	prev  State

	// Sub-models (composed, not inherited)
	list    ListModel
	spinner spinner.Model

	// Application state
	styles   Styles
	width    int
	height   int
	err      error
	info     *module.File
	infoItem *ModuleItem
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

// Update implements tea.Model - handles all messages.
//
//nolint:gocyclo // This is the central state-machine event handler for the TUI.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// Window resize - only forward to list if initialized
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.state == StateList {
			a.list, _ = a.list.Update(msg)
		}
		return a, nil

	// Keyboard input
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "b", "esc":
			if a.state == StateInfo {
				a.state = a.prev
				return a, nil
			}
			if a.state == StateSearch {
				a.state = StateList
				return a, nil
			}
		case "s", "/":
			if a.state == StateList {
				query := strings.TrimSpace(a.list.list.FilterValue())
				if query == "" {
					if item, ok := a.list.list.SelectedItem().(ModuleItem); ok {
						query = item.name
					}
				}
				if query != "" {
					a.prev = a.state
					a.state = StateSearch
					a.list = NewListModel(fmt.Sprintf("Search: %s", query), nil, a.styles)
					if a.width > 0 && a.height > 0 {
						a.list.SetSize(a.width, a.height)
					}
					return a, SearchModules(query)
				}
			}
		case "q":
			// Let list model handle q when in list/search state (including filter input).
			if a.state != StateList && a.state != StateSearch {
				return a, tea.Quit
			}
		}

	// Error handling
	case ErrMsg:
		a.state = StateError
		a.err = msg.Err
		return a, nil

	// Dependencies listed
	case DepsListedMsg:
		items := make([]ModuleItem, len(msg.File.Deps))
		for i, dep := range msg.File.Deps {
			items[i] = ModuleItem{
				name:    dep.Name.String(),
				version: dep.Version.String(),
				dev:     dep.DevDependency,
			}
		}

		title := "Dependencies"
		if msg.File.Name() != "" {
			title = fmt.Sprintf("%s - Dependencies", msg.File.Name())
		}

		a.list = NewListModel(title, items, a.styles)
		if a.width > 0 && a.height > 0 {
			a.list.SetSize(a.width, a.height)
		}
		a.state = StateList
		return a, nil

	// Module info fetched
	case ModuleInfoMsg:
		a.prev = a.state
		a.state = StateInfo
		a.info = msg.File
		a.infoItem = nil
		return a, nil

	// Module selected in list
	case ModuleSelectedMsg:
		item := msg.Item
		a.prev = a.state
		a.state = StateInfo
		a.info = nil
		a.infoItem = &item
		return a, nil

	// Search completed
	case SearchResultsMsg:
		items := make([]ModuleItem, len(msg.Results))
		for i, result := range msg.Results {
			items[i] = ModuleItem{
				name:    result.Name,
				version: result.Version,
			}
		}

		title := "Search Results"
		if msg.Query != "" {
			title = fmt.Sprintf("Search: %s", msg.Query)
		}
		if len(items) > 0 {
			title = fmt.Sprintf("%s (%d)", title, len(items))
		}

		a.list = NewListModel(title, items, a.styles)
		if a.width > 0 && a.height > 0 {
			a.list.SetSize(a.width, a.height)
		}
		a.state = StateSearch
		return a, nil

	// Spinner tick
	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Delegate to sub-models based on state
	switch a.state {
	case StateList, StateSearch:
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
	case StateSearch:
		return a.list.View()
	case StateError:
		return a.viewError()
	case StateInfo:
		return a.viewInfo()
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

func (a App) viewInfo() string {
	if a.infoItem != nil {
		var b strings.Builder
		name := a.infoItem.name
		if name == "" {
			name = "Module Info"
		}
		b.WriteString(a.styles.Title.Render(name))
		if a.infoItem.version != "" {
			b.WriteString(" ")
			b.WriteString(a.styles.Muted.Render("(" + a.infoItem.version + ")"))
		}
		b.WriteString("\n\n")
		b.WriteString("Selected dependency from MODULE.bazel\n")
		if a.infoItem.dev {
			b.WriteString("Dev Dependency: yes\n")
		} else {
			b.WriteString("Dev Dependency: no\n")
		}
		b.WriteString("\n")
		b.WriteString(a.styles.Help.Render("Press b to go back, q to quit"))
		return a.styles.App.Render(b.String())
	}

	if a.info == nil {
		return a.styles.App.Render(
			a.styles.Muted.Render("No module info available.") + "\n\n" +
				a.styles.Help.Render("Press b to go back, q to quit"),
		)
	}

	var b strings.Builder
	name := a.info.Name()
	if name == "" {
		name = "Module Info"
	}
	b.WriteString(a.styles.Title.Render(name))
	if v := a.info.Version(); v != "" {
		b.WriteString(" ")
		b.WriteString(a.styles.Muted.Render("(" + v + ")"))
	}
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Dependencies: %d\n", len(a.info.Deps)))
	b.WriteString(fmt.Sprintf("Extensions: %d\n", len(a.info.Extensions)))
	b.WriteString(fmt.Sprintf("Overrides: %d\n", len(a.info.Overrides)))

	if len(a.info.Deps) > 0 {
		b.WriteString("\n")
		b.WriteString("Dependency List:\n")
		for _, dep := range a.info.Deps {
			dev := ""
			if dep.DevDependency {
				dev = " (dev)"
			}
			b.WriteString(fmt.Sprintf("  - %s %s%s\n", dep.Name.String(), dep.Version.String(), dev))
		}
	}

	b.WriteString("\n")
	b.WriteString(a.styles.Help.Render("Press b to go back, q to quit"))
	return a.styles.App.Render(b.String())
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
		items := make([]ModuleItem, len(m.File.Deps))
		for i, dep := range m.File.Deps {
			items[i] = ModuleItem{
				name:    dep.Name.String(),
				version: dep.Version.String(),
				dev:     dep.DevDependency,
			}
		}

		if m.File.Name() != "" {
			fmt.Printf("%s", styles.Title.Render(m.File.Name()))
			if m.File.Version() != "" {
				fmt.Printf(" %s", styles.Muted.Render("("+m.File.Version()+")"))
			}
			fmt.Println()
		}

		fmt.Println(RenderDepsTable(items, styles))
	}

	return nil
}
