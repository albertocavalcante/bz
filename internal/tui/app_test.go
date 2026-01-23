package tui

import (
	"testing"

	gobzlmod "github.com/albertocavalcante/go-bzlmod"
	tea "github.com/charmbracelet/bubbletea"
)

func TestApp_WindowSizeMsg_BeforeListInitialized(t *testing.T) {
	// This test ensures we don't panic when WindowSizeMsg arrives
	// before the list model is initialized (during StateLoading)

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
	app := NewApp()

	// Simulate DepsListedMsg to initialize the list
	depsMsg := DepsListedMsg{
		Module: &gobzlmod.ModuleInfo{
			Name:    "test-module",
			Version: "1.0.0",
			Dependencies: []gobzlmod.Dependency{
				{Name: "rules_go", Version: "0.50.0"},
			},
		},
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
		Module: &gobzlmod.ModuleInfo{
			Name:    "test-module",
			Version: "1.0.0",
		},
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
	app := NewApp()

	errMsg := ErrMsg{Err: errTest}
	model, _ := app.Update(errMsg)
	app = model.(App)

	if app.state != StateError {
		t.Errorf("expected state StateError, got %v", app.state)
	}
	if app.err != errTest {
		t.Errorf("expected error %v, got %v", errTest, app.err)
	}
}

var errTest = &testError{msg: "test error"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
