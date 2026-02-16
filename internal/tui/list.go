package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModuleItem represents a module in the list
type ModuleItem struct {
	name    string
	version string
	dev     bool
}

func (i ModuleItem) Title() string       { return i.name }
func (i ModuleItem) Description() string { return i.version }
func (i ModuleItem) FilterValue() string { return i.name }

// ListModel is a reusable list component following Elm architecture
type ListModel struct {
	list     list.Model
	styles   Styles
	selected *ModuleItem
	quitting bool
}

const listViewportPadding = 4

// ListKeyMap defines keybindings for the list
type ListKeyMap struct {
	Select key.Binding
	Quit   key.Binding
}

var listKeys = ListKeyMap{
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q", "quit"),
	),
}

// NewListModel creates a new list model
func NewListModel(title string, items []ModuleItem, styles Styles) ListModel {
	// Convert to list.Item slice
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	// Configure the list
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("255")).
		Background(Primary).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("250")).
		Background(Primary)

	l := list.New(listItems, delegate, 60, 20)
	l.Title = title
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = styles.Title

	return ListModel{
		list:   l,
		styles: styles,
	}
}

// Init implements tea.Model
func (m ListModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, listKeys.Quit):
			// While editing filter input, q/esc should be handled by the list input.
			if m.list.SettingFilter() {
				break
			}
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, listKeys.Select):
			if item, ok := m.list.SelectedItem().(ModuleItem); ok {
				m.selected = &item
				return m, func() tea.Msg {
					return ModuleSelectedMsg{Item: item}
				}
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		w, h := clampListSize(msg.Width, msg.Height)
		m.list.SetSize(w, h)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model
func (m ListModel) View() string {
	if m.quitting {
		return ""
	}
	return m.styles.App.Render(m.list.View())
}

// Selected returns the selected item, if any
func (m ListModel) Selected() *ModuleItem {
	return m.selected
}

// SetItems updates the list items
func (m *ListModel) SetItems(items []ModuleItem) tea.Cmd {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}
	return m.list.SetItems(listItems)
}

// SetSize sets the list dimensions
func (m *ListModel) SetSize(width, height int) {
	w, h := clampListSize(width, height)
	m.list.SetSize(w, h)
}

func clampListSize(width, height int) (int, int) {
	return max(0, width-listViewportPadding), max(0, height-listViewportPadding)
}

// RenderDepsTable renders dependencies as a simple table (for headless mode)
func RenderDepsTable(deps []ModuleItem, styles Styles) string {
	if len(deps) == 0 {
		return styles.Muted.Render("No dependencies found.")
	}

	var b strings.Builder

	// Header
	header := fmt.Sprintf("%-30s %-15s %s", "NAME", "VERSION", "DEV")
	b.WriteString(styles.Muted.Render(header))
	b.WriteString("\n")

	// Rows
	for _, dep := range deps {
		dev := ""
		if dep.dev {
			dev = "yes"
		}
		row := fmt.Sprintf("%-30s %-15s %s", dep.name, dep.version, dev)
		b.WriteString(styles.Normal.Render(row))
		b.WriteString("\n")
	}

	return b.String()
}
