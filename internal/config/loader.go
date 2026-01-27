package config

import (
	"fmt"
	"os"
	"path/filepath"

	"go.starlark.net/starlark"

	"github.com/albertocavalcante/bz/internal/config/modules"
	bzstarlark "github.com/albertocavalcante/bz/internal/starlark"
	"github.com/albertocavalcante/bz/internal/starlark/builtins"
)

// Default configuration file locations.
var defaultConfigPaths = []string{
	"bz.star",
	".bz/config.star",
}

// userConfigPath returns the user-level config path.
func userConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "bz", "config.star")
}

// Loader loads bz.star configuration files.
type Loader struct {
	// workingDir is the directory to search for config files
	workingDir string
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	return &Loader{}
}

// WithWorkingDir sets the working directory for config file resolution.
func (l *Loader) WithWorkingDir(dir string) *Loader {
	l.workingDir = dir
	return l
}

// Load loads and parses a bz.star file.
func (l *Loader) Load(filename string) (*Config, error) {
	// Resolve to absolute path
	absPath := filename
	if !filepath.IsAbs(filename) {
		if l.workingDir != "" {
			absPath = filepath.Join(l.workingDir, filename)
		} else {
			var err error
			absPath, err = filepath.Abs(filename)
			if err != nil {
				return nil, fmt.Errorf("resolving path: %w", err)
			}
		}
	}

	// Check if file exists
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("config file not found: %s", absPath)
	}

	// Create interpreter with all bz modules and builtins
	interp := l.createInterpreter()

	// Set up thread-local storage for workflows and config defaults
	thread := interp.Thread()
	modules.SetWorkflowRegistry(thread)
	modules.SetConfigDefaults(thread)

	// Execute the file
	_, err := interp.ExecFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("executing config file: %w", err)
	}

	// Extract configuration from thread-local storage
	return l.extractConfig(thread), nil
}

// LoadDefault loads from default locations:
// 1. ./bz.star (current dir)
// 2. ./.bz/config.star
// 3. ~/.config/bz/config.star
func (l *Loader) LoadDefault() (*Config, error) {
	// Determine base directory
	baseDir := l.workingDir
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting working directory: %w", err)
		}
	}

	// Check local config paths
	for _, relPath := range defaultConfigPaths {
		absPath := filepath.Join(baseDir, relPath)
		if _, err := os.Stat(absPath); err == nil {
			return l.Load(absPath)
		}
	}

	// Check user config path
	userPath := userConfigPath()
	if userPath != "" {
		if _, err := os.Stat(userPath); err == nil {
			return l.Load(userPath)
		}
	}

	return nil, fmt.Errorf("no config file found in default locations: %v, %s", defaultConfigPaths, userPath)
}

// createInterpreter creates a Starlark interpreter with all bz modules and builtins.
func (l *Loader) createInterpreter() *bzstarlark.Interpreter {
	opts := []bzstarlark.Option{}

	// Add all bz modules
	for name, module := range modules.All() {
		opts = append(opts, bzstarlark.WithModule(name, module))
	}

	// Add all builtins (type assert from starlark.Value to *starlark.Builtin)
	for name, builtin := range builtins.All() {
		if b, ok := builtin.(*starlark.Builtin); ok {
			opts = append(opts, bzstarlark.WithBuiltin(name, b))
		}
	}

	return bzstarlark.New(opts...)
}

// extractConfig extracts the configuration from thread-local storage.
func (l *Loader) extractConfig(thread *starlark.Thread) *Config {
	cfg := &Config{}

	// Extract workflows using the module helper
	workflows := modules.GetWorkflows(thread)
	if len(workflows) > 0 {
		cfg.Workflows = make([]*Workflow, len(workflows))
		for i, wf := range workflows {
			cfg.Workflows[i] = WorkflowFromValue(wf)
		}
	}

	// Extract config defaults using the module helper
	defaults := modules.GetConfigDefaults(thread)
	if defaults != nil {
		cfg.Defaults = DefaultsFromValue(defaults)
	}

	return cfg
}
