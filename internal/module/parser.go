// Package module provides utilities for parsing and manipulating MODULE.bazel files.
package module

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Dep represents a bazel_dep entry in MODULE.bazel.
type Dep struct {
	Name       string
	Version    string
	DevOnly    bool
	RepoName   string
	LineNumber int
}

// Module represents a parsed MODULE.bazel file.
type Module struct {
	Name    string
	Version string
	Deps    []Dep
	Path    string
}

var bazelDepRegex = regexp.MustCompile(`bazel_dep\s*\(\s*name\s*=\s*"([^"]+)"(?:\s*,\s*version\s*=\s*"([^"]+)")?`)

// Parse reads and parses a MODULE.bazel file.
func Parse(path string) (*Module, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open MODULE.bazel: %w", err)
	}
	defer file.Close()

	m := &Module{
		Path: path,
		Deps: make([]Dep, 0),
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Parse module() call for name/version
		if strings.HasPrefix(strings.TrimSpace(line), "module(") {
			// TODO: more robust parsing
			if matches := regexp.MustCompile(`name\s*=\s*"([^"]+)"`).FindStringSubmatch(line); len(matches) > 1 {
				m.Name = matches[1]
			}
			if matches := regexp.MustCompile(`version\s*=\s*"([^"]+)"`).FindStringSubmatch(line); len(matches) > 1 {
				m.Version = matches[1]
			}
		}

		// Parse bazel_dep() calls
		if matches := bazelDepRegex.FindStringSubmatch(line); len(matches) > 1 {
			dep := Dep{
				Name:       matches[1],
				LineNumber: lineNum,
			}
			if len(matches) > 2 {
				dep.Version = matches[2]
			}
			if strings.Contains(line, "dev_dependency = True") {
				dep.DevOnly = true
			}
			m.Deps = append(m.Deps, dep)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading MODULE.bazel: %w", err)
	}

	return m, nil
}

// FindModuleFile looks for MODULE.bazel in the current directory and parent directories.
func FindModuleFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := dir + "/MODULE.bazel"
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}

		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir || parent == "" {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("MODULE.bazel not found")
}
