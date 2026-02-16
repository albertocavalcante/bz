package tui

import (
	"strings"
	"testing"

	listpkg "github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewListModel_DefaultConfiguration(t *testing.T) {
	t.Parallel()

	items := []ModuleItem{
		{name: "rules_go", version: "0.50.0"},
		{name: "gazelle", version: "0.38.0"},
	}
	model := NewListModel("Dependencies", items, DefaultStyles())

	if got := model.list.Title; got != "Dependencies" {
		t.Fatalf("Title = %q, want %q", got, "Dependencies")
	}
	if !model.list.ShowStatusBar() {
		t.Fatal("expected status bar to be enabled")
	}
	if !model.list.FilteringEnabled() {
		t.Fatal("expected filtering to be enabled")
	}
	if got := len(model.list.Items()); got != len(items) {
		t.Fatalf("len(Items()) = %d, want %d", got, len(items))
	}
}

func TestListModel_Update_NavigationAndSelection(t *testing.T) {
	t.Parallel()

	model := NewListModel("Dependencies", []ModuleItem{
		{name: "rules_go", version: "0.50.0"},
		{name: "gazelle", version: "0.38.0"},
	}, DefaultStyles())

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := updated.list.Index(); got != 1 {
		t.Fatalf("Index() after down = %d, want %d", got, 1)
	}

	var cmd tea.Cmd
	updated, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected := updated.Selected()
	if selected == nil {
		t.Fatal("expected selected item, got nil")
	}
	if selected.name != "gazelle" {
		t.Fatalf("selected.name = %q, want %q", selected.name, "gazelle")
	}
	if cmd == nil {
		t.Fatal("expected command on selection")
	}
	msg := cmd()
	selectedMsg, ok := msg.(ModuleSelectedMsg)
	if !ok {
		t.Fatalf("expected ModuleSelectedMsg, got %T", msg)
	}
	if selectedMsg.Item.name != "gazelle" {
		t.Fatalf("selected message item = %q, want %q", selectedMsg.Item.name, "gazelle")
	}
}

func TestListModel_Update_FilterFlow(t *testing.T) {
	t.Parallel()

	model := NewListModel("Dependencies", []ModuleItem{
		{name: "rules_go", version: "0.50.0"},
		{name: "gazelle", version: "0.38.0"},
		{name: "rules_python", version: "0.40.0"},
	}, DefaultStyles())

	model.list.SetFilterText("rules")

	if got := model.list.FilterValue(); got != "rules" {
		t.Fatalf("FilterValue() = %q, want %q", got, "rules")
	}
	if got := model.list.FilterState(); got != listpkg.FilterApplied {
		t.Fatalf("FilterState() = %v, want %v", got, listpkg.FilterApplied)
	}
	if got := len(model.list.VisibleItems()); got != 2 {
		t.Fatalf("len(VisibleItems()) = %d, want %d", got, 2)
	}
}

func TestListModel_Update_QuitFlow(t *testing.T) {
	t.Parallel()

	model := NewListModel("Dependencies", []ModuleItem{
		{name: "rules_go", version: "0.50.0"},
	}, DefaultStyles())

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if !updated.quitting {
		t.Fatal("expected quitting = true after q")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg from quit command")
	}
	if got := updated.View(); got != "" {
		t.Fatalf("View() when quitting = %q, want empty", got)
	}
}

func TestListModel_Update_DoesNotQuitWhileSettingFilter(t *testing.T) {
	t.Parallel()

	model := NewListModel("Dependencies", []ModuleItem{
		{name: "rules_go", version: "0.50.0"},
	}, DefaultStyles())
	model.list.SetFilterState(listpkg.Filtering)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if updated.quitting {
		t.Fatal("expected quitting = false while setting filter")
	}
	if got := updated.list.FilterValue(); got != "q" {
		t.Fatalf("FilterValue() = %q, want %q", got, "q")
	}
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("unexpected tea.QuitMsg while setting filter")
		}
	}
}

func TestListModel_WindowSizeAndSetSize(t *testing.T) {
	t.Parallel()

	model := NewListModel("Dependencies", []ModuleItem{
		{name: "rules_go", version: "0.50.0"},
	}, DefaultStyles())

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if got := updated.list.Width(); got != 96 {
		t.Fatalf("Width() after WindowSizeMsg = %d, want %d", got, 96)
	}
	if got := updated.list.Height(); got != 36 {
		t.Fatalf("Height() after WindowSizeMsg = %d, want %d", got, 36)
	}

	updated.SetSize(80, 30)
	if got := updated.list.Width(); got != 76 {
		t.Fatalf("Width() after SetSize = %d, want %d", got, 76)
	}
	if got := updated.list.Height(); got != 26 {
		t.Fatalf("Height() after SetSize = %d, want %d", got, 26)
	}
}

func TestListModel_SizeIsClampedForSmallTerminal(t *testing.T) {
	t.Parallel()

	model := NewListModel("Dependencies", []ModuleItem{
		{name: "rules_go", version: "0.50.0"},
	}, DefaultStyles())

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 2, Height: 3})
	if got := updated.list.Width(); got != 0 {
		t.Fatalf("Width() after tiny WindowSizeMsg = %d, want %d", got, 0)
	}
	if got := updated.list.Height(); got != 0 {
		t.Fatalf("Height() after tiny WindowSizeMsg = %d, want %d", got, 0)
	}

	updated.SetSize(1, 1)
	if got := updated.list.Width(); got != 0 {
		t.Fatalf("Width() after tiny SetSize = %d, want %d", got, 0)
	}
	if got := updated.list.Height(); got != 0 {
		t.Fatalf("Height() after tiny SetSize = %d, want %d", got, 0)
	}
}

func TestListModel_SetItems_ReplacesItems(t *testing.T) {
	t.Parallel()

	model := NewListModel("Dependencies", []ModuleItem{
		{name: "rules_go", version: "0.50.0"},
	}, DefaultStyles())

	cmd := model.SetItems([]ModuleItem{
		{name: "gazelle", version: "0.38.0"},
		{name: "rules_python", version: "0.40.0"},
	})
	if cmd != nil {
		_ = cmd()
	}

	items := model.list.Items()
	if got := len(items); got != 2 {
		t.Fatalf("len(Items()) = %d, want %d", got, 2)
	}
	first, ok := items[0].(ModuleItem)
	if !ok {
		t.Fatalf("first item type = %T, want ModuleItem", items[0])
	}
	if first.name != "gazelle" {
		t.Fatalf("first item name = %q, want %q", first.name, "gazelle")
	}
}

func TestRenderDepsTable(t *testing.T) {
	t.Parallel()
	styles := DefaultStyles()

	t.Run("empty dependencies", func(t *testing.T) {
		t.Parallel()
		output := RenderDepsTable(nil, styles)
		if !strings.Contains(output, "No dependencies found.") {
			t.Fatalf("expected empty message, got %q", output)
		}
	})

	t.Run("renders header and rows", func(t *testing.T) {
		t.Parallel()
		output := RenderDepsTable([]ModuleItem{
			{name: "rules_go", version: "0.50.0"},
			{name: "gazelle", version: "0.38.0", dev: true},
		}, styles)

		if !strings.Contains(output, "NAME") || !strings.Contains(output, "VERSION") {
			t.Fatalf("expected header columns, got %q", output)
		}
		if !strings.Contains(output, "rules_go") || !strings.Contains(output, "gazelle") {
			t.Fatalf("expected dependency rows, got %q", output)
		}
		if !strings.Contains(output, "yes") {
			t.Fatalf("expected dev marker 'yes', got %q", output)
		}
	})
}
