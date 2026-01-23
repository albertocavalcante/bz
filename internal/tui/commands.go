package tui

import (
	"context"
	"fmt"
	"os"

	gobzlmod "github.com/albertocavalcante/go-bzlmod"
	tea "github.com/charmbracelet/bubbletea"
)

const defaultRegistry = "https://bcr.bazel.build"

// FetchModuleInfo fetches module info from the registry
func FetchModuleInfo(name, version string) tea.Cmd {
	return func() tea.Msg {
		client := gobzlmod.NewRegistryClient(defaultRegistry)
		info, err := client.GetModuleFile(context.Background(), name, version)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to fetch %s@%s: %w", name, version, err)}
		}
		return ModuleInfoMsg{Info: info}
	}
}

// ListLocalDeps reads and parses the local MODULE.bazel
func ListLocalDeps() tea.Cmd {
	return func() tea.Msg {
		path := "MODULE.bazel"
		if _, err := os.Stat(path); err != nil {
			return ErrMsg{Err: fmt.Errorf("MODULE.bazel not found in current directory")}
		}

		info, err := gobzlmod.ParseModuleFile(path)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to parse MODULE.bazel: %w", err)}
		}
		return DepsListedMsg{Module: info}
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
		path := "MODULE.bazel"
		content, err := os.ReadFile(path)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to read MODULE.bazel: %w", err)}
		}

		// Build new bazel_dep line
		var depLine string
		if dev {
			depLine = fmt.Sprintf("bazel_dep(name = \"%s\", version = \"%s\", dev_dependency = True)\n", name, version)
		} else {
			depLine = fmt.Sprintf("bazel_dep(name = \"%s\", version = \"%s\")\n", name, version)
		}

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
