package tui

import (
	"fmt"
	"os"

	"github.com/albertocavalcante/bz/internal/module"
	tea "github.com/charmbracelet/bubbletea"
)

// ListLocalDeps reads and parses the local MODULE.bazel
func ListLocalDeps() tea.Cmd {
	return func() tea.Msg {
		f, err := module.FindAndLoad()
		if err != nil {
			return ErrMsg{Err: err}
		}
		return DepsListedMsg{File: f}
	}
}

// SearchModules searches for modules (placeholder - BCR doesn't have search API)
// In reality, you'd need to fetch the module index or use a different approach
func SearchModules(query string) tea.Cmd {
	return func() tea.Msg {
		// BCR doesn't have a search API, so this is a placeholder
		// You could fetch https://bcr.bazel.build/bazel_registry.json for the index
		return SearchResultsMsg{
			Query:   query,
			Results: []ModuleResult{},
		}
	}
}

// AddDependency adds a dependency to MODULE.bazel
func AddDependency(name, version string, dev bool) tea.Cmd {
	return func() tea.Msg {
		path, err := module.Find()
		if err != nil {
			return ErrMsg{Err: err}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to read MODULE.bazel: %w", err)}
		}

		depLine := module.FormatBazelDep(name, version, dev)

		newContent := string(content)
		if len(newContent) > 0 && newContent[len(newContent)-1] != '\n' {
			newContent += "\n"
		}
		newContent += depLine

		if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to write MODULE.bazel: %w", err)}
		}

		return DepAddedMsg{Name: name, Version: version}
	}
}
