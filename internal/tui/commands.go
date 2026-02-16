package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
)

const searchTimeout = 15 * time.Second

var getSearchRegistry = defaultSearchRegistry
var findAndLoadModule = module.FindAndLoad
var initModuleInDir = module.InitInDir
var getWorkingDir = os.Getwd

// ListLocalDeps reads and parses the local MODULE.bazel
func ListLocalDeps() tea.Cmd {
	return func() tea.Msg {
		f, err := findAndLoadModule()
		if err != nil {
			if errors.Is(err, module.ErrModuleFileNotFound) {
				return DepsListedMsg{
					File:          &module.File{},
					MissingModule: true,
				}
			}
			return ErrMsg{Err: err}
		}
		return DepsListedMsg{File: f}
	}
}

// InitModule initializes a MODULE.bazel in the current working directory.
func InitModule(name, version string, force bool) tea.Cmd {
	return func() tea.Msg {
		wd, err := getWorkingDir()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to get working directory: %w", err)}
		}

		path, resolvedName, err := initModuleInDir(wd, name, version, force)
		if err != nil {
			return ErrMsg{Err: err}
		}

		if version == "" {
			version = module.DefaultModuleVersion
		}

		return ModuleInitializedMsg{
			Name:    resolvedName,
			Version: version,
			Path:    path,
		}
	}
}

func defaultSearchRegistry() (registry.Registry, error) {
	url := cli.GetRegistry()
	if url == "" {
		return registry.Default(), nil
	}

	reg, err := registry.New(url)
	if err != nil {
		return nil, fmt.Errorf("invalid registry %q: %w", url, err)
	}
	return reg, nil
}

func searchModules(ctx context.Context, reg registry.Registry, query string) ([]ModuleResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []ModuleResult{}, nil
	}

	results, err := registry.Search(ctx, reg, query)
	if err != nil {
		return nil, err
	}

	out := make([]ModuleResult, 0, len(results))
	for _, result := range results {
		item := ModuleResult{Name: result.Name}
		if meta, metaErr := reg.GetMetadata(ctx, result.Name); metaErr == nil && meta != nil {
			item.Version = meta.LatestVersion()
		}
		out = append(out, item)
	}

	return out, nil
}

// SearchModules searches for modules from the configured registry.
func SearchModules(query string) tea.Cmd {
	return func() tea.Msg {
		reg, err := getSearchRegistry()
		if err != nil {
			return ErrMsg{Err: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
		defer cancel()

		results, err := searchModules(ctx, reg, query)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("search modules: %w", err)}
		}

		return SearchResultsMsg{
			Query:   strings.TrimSpace(query),
			Results: results,
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
