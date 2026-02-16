package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/albertocavalcante/go-bzlmod/ast"
	"github.com/albertocavalcante/go-bzlmod/label"
	listpkg "github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/albertocavalcante/bz/internal/module"
)

// testModuleFile creates a test module.File with the given name, version, and deps
func testModuleFile(name, version string, deps ...struct{ name, ver string }) *module.File {
	f := &module.File{}

	if name != "" {
		f.Module = &ast.ModuleDecl{
			Name:    label.MustModule(name),
			Version: label.MustVersion(version),
		}
	}

	for _, d := range deps {
		f.Deps = append(f.Deps, &ast.BazelDep{
			Name:    label.MustModule(d.name),
			Version: label.MustVersion(d.ver),
		})
	}

	return f
}

func TestApp_WindowSizeMsg_BeforeListInitialized(t *testing.T) {
	t.Parallel()
	// This test ensures we don't panic when WindowSizeMsg arrives
	// before the list model is initialized (during StateLoading).

	app := NewApp()

	// Verify initial state
	if app.state != StateLoading {
		t.Errorf("expected initial state StateLoading, got %v", app.state)
	}

	// Send WindowSizeMsg while still in StateLoading - this should NOT panic
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	model, _ := app.Update(msg)

	updatedApp, ok := model.(App)
	if !ok {
		t.Fatal("expected App model")
	}

	// Verify dimensions were stored
	if updatedApp.width != 100 {
		t.Errorf("expected width 100, got %d", updatedApp.width)
	}
	if updatedApp.height != 50 {
		t.Errorf("expected height 50, got %d", updatedApp.height)
	}

	// Verify still in loading state
	if updatedApp.state != StateLoading {
		t.Errorf("expected state StateLoading, got %v", updatedApp.state)
	}
}

func TestApp_WindowSizeMsg_AfterListInitialized(t *testing.T) {
	t.Parallel()
	app := NewApp()

	// Simulate DepsListedMsg to initialize the list
	depsMsg := DepsListedMsg{
		File: testModuleFile("test-module", "1.0.0",
			struct{ name, ver string }{"rules_go", "0.50.0"},
		),
	}
	model, _ := app.Update(depsMsg)
	app = model.(App)

	// Verify we're now in StateList
	if app.state != StateList {
		t.Errorf("expected state StateList, got %v", app.state)
	}

	// Now send WindowSizeMsg - should work without panic
	msg := tea.WindowSizeMsg{Width: 120, Height: 60}
	model, _ = app.Update(msg)
	updatedApp := model.(App)

	if updatedApp.width != 120 {
		t.Errorf("expected width 120, got %d", updatedApp.width)
	}
	if updatedApp.height != 60 {
		t.Errorf("expected height 60, got %d", updatedApp.height)
	}
}

func TestApp_DepsListedMsg_AppliesStoredDimensions(t *testing.T) {
	t.Parallel()
	app := NewApp()

	// First, send WindowSizeMsg to store dimensions
	sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	model, _ := app.Update(sizeMsg)
	app = model.(App)

	// Verify dimensions stored
	if app.width != 100 || app.height != 50 {
		t.Errorf("expected dimensions 100x50, got %dx%d", app.width, app.height)
	}

	// Now send DepsListedMsg
	depsMsg := DepsListedMsg{
		File: testModuleFile("test-module", "1.0.0"),
	}
	model, _ = app.Update(depsMsg)
	app = model.(App)

	// Verify state changed to StateList
	if app.state != StateList {
		t.Errorf("expected state StateList, got %v", app.state)
	}

	// The list should have been initialized with the stored dimensions
	// (We verify no panic occurred and state transitioned correctly)
}

func TestApp_ErrMsg_TransitionsToErrorState(t *testing.T) {
	t.Parallel()
	app := NewApp()

	errMsg := ErrMsg{Err: errTest}
	model, _ := app.Update(errMsg)
	app = model.(App)

	if app.state != StateError {
		t.Errorf("expected state StateError, got %v", app.state)
	}
	if !errors.Is(app.err, errTest) {
		t.Errorf("expected error %v, got %v", errTest, app.err)
	}
}

func TestApp_KeyQuitAndCtrlC_ReturnQuitCmd(t *testing.T) {
	t.Parallel()
	app := NewApp()

	tests := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyCtrlC},
	}

	for _, keyMsg := range tests {
		model, cmd := app.Update(keyMsg)
		updated := model.(App)
		if updated.state != StateLoading {
			t.Fatalf("state changed unexpectedly for key %q: %v", keyMsg.String(), updated.state)
		}

		if cmd == nil {
			t.Fatalf("expected quit cmd for key %q", keyMsg.String())
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("expected tea.QuitMsg for key %q", keyMsg.String())
		}
	}
}

func TestApp_KeyQ_InListFilterInput_DoesNotQuit(t *testing.T) {
	t.Parallel()
	app := NewApp()

	model, _ := app.Update(DepsListedMsg{
		File: testModuleFile("demo", "1.0.0",
			struct{ name, ver string }{"rules_go", "0.50.0"},
		),
	})
	app = model.(App)
	app.list.list.SetFilterState(listpkg.Filtering)

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	updated := model.(App)

	if updated.state != StateList {
		t.Fatalf("expected state StateList, got %v", updated.state)
	}
	if updated.list.quitting {
		t.Fatal("expected list not to quit while setting filter")
	}
	if got := updated.list.list.FilterValue(); got != "q" {
		t.Fatalf("FilterValue() = %q, want %q", got, "q")
	}
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("unexpected tea.QuitMsg from q while setting filter")
		}
	}
}

func TestApp_ListSelection_TransitionsToInfoState(t *testing.T) {
	t.Parallel()
	app := NewApp()

	model, _ := app.Update(DepsListedMsg{
		File: testModuleFile("demo", "1.0.0",
			struct{ name, ver string }{"rules_go", "0.50.0"},
			struct{ name, ver string }{"gazelle", "0.38.0"},
		),
	})
	app = model.(App)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app = model.(App)

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(App)
	if cmd == nil {
		t.Fatal("expected command on enter selection")
	}

	msg := cmd()
	selectedMsg, ok := msg.(ModuleSelectedMsg)
	if !ok {
		t.Fatalf("expected ModuleSelectedMsg, got %T", msg)
	}
	if selectedMsg.Item.name != "gazelle" {
		t.Fatalf("selected item = %q, want %q", selectedMsg.Item.name, "gazelle")
	}

	model, _ = app.Update(msg)
	app = model.(App)
	if app.state != StateInfo {
		t.Fatalf("expected state StateInfo, got %v", app.state)
	}
	view := app.View()
	if !strings.Contains(view, "gazelle") {
		t.Fatalf("info view missing selected module name: %q", view)
	}
	if !strings.Contains(view, "Selected dependency from MODULE.bazel") {
		t.Fatalf("info view missing selected dependency marker: %q", view)
	}

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	app = model.(App)
	if app.state != StateList {
		t.Fatalf("expected state StateList after back, got %v", app.state)
	}
}

func TestApp_DepsListedMsg_UsesFallbackTitleWithoutModuleName(t *testing.T) {
	t.Parallel()
	app := NewApp()

	msg := DepsListedMsg{
		File: testModuleFile("", "",
			struct{ name, ver string }{"rules_go", "0.50.0"},
		),
	}
	model, _ := app.Update(msg)
	updated := model.(App)

	if updated.state != StateList {
		t.Fatalf("expected state StateList, got %v", updated.state)
	}
	if got := updated.list.list.Title; got != "Dependencies" {
		t.Fatalf("list title = %q, want %q", got, "Dependencies")
	}
}

func TestApp_ModuleInfoMsg_TransitionsToInfoState(t *testing.T) {
	t.Parallel()
	app := NewApp()

	model, _ := app.Update(ModuleInfoMsg{
		File: testModuleFile("rules_go", "0.50.0"),
	})
	updated := model.(App)

	if updated.state != StateInfo {
		t.Fatalf("expected state StateInfo, got %v", updated.state)
	}
	view := updated.View()
	if !strings.Contains(view, "rules_go") {
		t.Fatalf("info view missing module name: %q", view)
	}
	if !strings.Contains(view, "Dependencies: 0") {
		t.Fatalf("info view missing dependency summary: %q", view)
	}
	if !strings.Contains(view, "Press b to go back, q to quit") {
		t.Fatalf("info view missing help text: %q", view)
	}
}

func TestApp_InfoBackNavigation_RestoresPreviousState(t *testing.T) {
	t.Parallel()
	app := NewApp()

	model, _ := app.Update(DepsListedMsg{
		File: testModuleFile("demo", "1.0.0",
			struct{ name, ver string }{"rules_go", "0.50.0"},
		),
	})
	app = model.(App)
	if app.state != StateList {
		t.Fatalf("expected state StateList, got %v", app.state)
	}

	model, _ = app.Update(ModuleInfoMsg{
		File: testModuleFile("rules_go", "0.50.0"),
	})
	app = model.(App)
	if app.state != StateInfo {
		t.Fatalf("expected state StateInfo, got %v", app.state)
	}

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	app = model.(App)
	if app.state != StateList {
		t.Fatalf("expected state StateList after back, got %v", app.state)
	}
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("unexpected tea.QuitMsg from info back key")
		}
	}
}

func TestApp_ViewInfo_NoModuleData(t *testing.T) {
	t.Parallel()
	app := NewApp()

	model, _ := app.Update(ModuleInfoMsg{})
	view := model.(App).View()
	if !strings.Contains(view, "No module info available.") {
		t.Fatalf("info view missing empty-data text: %q", view)
	}
}

func TestApp_View_RendersLoadingAndError(t *testing.T) {
	t.Parallel()

	loading := NewApp().View()
	if !strings.Contains(loading, "Loading...") {
		t.Fatalf("loading view missing marker: %q", loading)
	}

	app := NewApp()
	model, _ := app.Update(ErrMsg{Err: errTest})
	errView := model.(App).View()
	if !strings.Contains(errView, "Error: test error") {
		t.Fatalf("error view missing error text: %q", errView)
	}
	if !strings.Contains(errView, "Press q to quit") {
		t.Fatalf("error view missing help text: %q", errView)
	}
}

func TestApp_View_RendersListAfterDepsListed(t *testing.T) {
	t.Parallel()
	app := NewApp()

	model, _ := app.Update(DepsListedMsg{
		File: testModuleFile("demo", "1.0.0",
			struct{ name, ver string }{"rules_go", "0.50.0"},
			struct{ name, ver string }{"gazelle", "0.38.0"},
		),
	})
	view := model.(App).View()

	if !strings.Contains(view, "rules_go") {
		t.Fatalf("list view missing dependency: %q", view)
	}
	if !strings.Contains(view, "gazelle") {
		t.Fatalf("list view missing dependency: %q", view)
	}
}

var errTest = &testError{msg: "test error"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
