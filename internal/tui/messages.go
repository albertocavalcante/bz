package tui

import "github.com/albertocavalcante/bz/internal/module"

// Message types following Elm architecture
// All messages should be defined here for type safety

// ErrMsg represents an error that occurred during a command
type ErrMsg struct {
	Err error
}

func (e ErrMsg) Error() string {
	return e.Err.Error()
}

// ModulesFetchedMsg is sent when modules are fetched from the registry
type ModulesFetchedMsg struct {
	Modules []ModuleResult
}

// ModuleResult represents a module from search or info
type ModuleResult struct {
	Name        string
	Version     string
	Description string
	Versions    []string
}

// ModuleInfoMsg is sent when module info is fetched
type ModuleInfoMsg struct {
	File *module.File
}

// ModuleSelectedMsg is sent when a module/dependency is selected in the list.
type ModuleSelectedMsg struct {
	Item ModuleItem
}

// DepsListedMsg is sent when local deps are listed
type DepsListedMsg struct {
	File          *module.File
	MissingModule bool
}

// VersionsMsg is sent when versions are fetched for a module
type VersionsMsg struct {
	Name     string
	Versions []string
}

// DepAddedMsg is sent when a dependency is successfully added
type DepAddedMsg struct {
	Name    string
	Version string
}

// ModuleInitializedMsg is sent when MODULE.bazel is initialized.
type ModuleInitializedMsg struct {
	Name    string
	Version string
	Path    string
}

// SearchResultsMsg is sent when search results are ready
type SearchResultsMsg struct {
	Query   string
	Results []ModuleResult
}

// QuitMsg signals the app should quit
type QuitMsg struct{}
