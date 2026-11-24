package cli

import (
	"fmt"
	"slices"
	"sync"

	"github.com/albertocavalcante/bz/internal/bzconfig"
)

// The file and environment layers are read once per process (the root
// command's pre-run calls LoadEffective); the flag layer is overlaid from
// Global on each access so commands always see the parsed flags.
var (
	configMu  sync.Mutex
	configEnv = bzconfig.OSEnvironment()
	loaded    *bzconfig.Loaded
	loadErr   error
	loadDone  bool
)

// SetEnvironment replaces the environment configuration is read from and
// drops any cached configuration. It returns a function restoring the
// previous environment. Tests use it to keep the real home directory and
// /etc out of the picture.
func SetEnvironment(env bzconfig.Environment) (restore func()) {
	configMu.Lock()
	prev := configEnv
	configEnv = env
	loaded, loadErr, loadDone = nil, nil, false
	configMu.Unlock()
	return func() {
		configMu.Lock()
		configEnv = prev
		loaded, loadErr, loadDone = nil, nil, false
		configMu.Unlock()
	}
}

// ResetCachedConfig drops the cached file and environment layers so they
// are read again on next access. This is intended for use in tests.
func ResetCachedConfig() {
	configMu.Lock()
	loaded, loadErr, loadDone = nil, nil, false
	configMu.Unlock()
}

func loadLayers() (*bzconfig.Loaded, error) {
	configMu.Lock()
	defer configMu.Unlock()
	if !loadDone {
		loaded, loadErr = bzconfig.LoadLayers(configEnv)
		loadDone = true
	}
	return loaded, loadErr
}

func flagLayer() bzconfig.Layer {
	return bzconfig.FlagLayer(Global.Offline, Global.PreferOffline, Global.Registry)
}

// LoadEffective reads every configuration layer afresh and returns the
// effective configuration together with warnings to print once (skipped
// workspace markers, unusable BZ_TRUSTED_CONFIG_DIRS entries). The root
// command calls it once per invocation before running a command.
func LoadEffective() (*bzconfig.Effective, []string, error) {
	ResetCachedConfig()
	l, err := loadLayers()
	var warnings []string
	if l != nil {
		warnings = l.Warnings
	}
	if err != nil {
		return nil, warnings, err
	}
	eff, err := l.Resolve(flagLayer())
	return eff, warnings, err
}

// Effective returns the effective configuration: defaults < system file <
// user file < project file < environment < global flags.
func Effective() (*bzconfig.Effective, error) {
	l, err := loadLayers()
	if err != nil {
		return nil, err
	}
	return l.Resolve(flagLayer())
}

// CommandDisabledError is returned when a command is disabled in configuration.
type CommandDisabledError struct {
	Command string
	Reason  string
}

func (e *CommandDisabledError) Error() string {
	return fmt.Sprintf("command %q is disabled: %s", e.Command, e.Reason)
}

// Is implements errors.Is for CommandDisabledError.
func (e *CommandDisabledError) Is(target error) bool {
	_, ok := target.(*CommandDisabledError)
	return ok
}

// CheckCommandAllowed checks if a command is allowed based on configuration.
// Returns CommandDisabledError if the command is disabled. It fails closed:
// when the configuration cannot be loaded, the load error is returned and
// the command must not run.
func CheckCommandAllowed(cmdName string) error {
	cfg, err := Effective()
	if err != nil {
		return err
	}

	if slices.Contains(cfg.Commands.Disabled, cmdName) {
		return &CommandDisabledError{
			Command: cmdName,
			Reason:  "disabled in configuration",
		}
	}

	return nil
}

// OfflineModeError is returned when a command requires network access but offline mode is enabled.
type OfflineModeError struct {
	Command     string
	Description string
}

func (e *OfflineModeError) Error() string {
	msg := fmt.Sprintf("'bz %s' requires network access", e.Command)
	if e.Description != "" {
		msg += " " + e.Description
	}
	msg += ".\n\nIn offline mode, this operation is not available.\n\nOptions:\n"
	msg += "  1. Run with network access (remove --offline flag or BZ_OFFLINE env var)\n"
	msg += fmt.Sprintf("  2. Disable this command in .bzconfig.toml:\n     [commands]\n     disabled = [%q]\n", e.Command)
	return msg
}

// Is implements errors.Is for OfflineModeError.
func (e *OfflineModeError) Is(target error) bool {
	_, ok := target.(*OfflineModeError)
	return ok
}

// CheckOfflineAllowed checks if network access is available for a command.
// Returns OfflineModeError if offline mode is enabled.
// This checks both CLI flags and configuration.
func CheckOfflineAllowed(cmdName string) error {
	if IsEffectivelyOffline() {
		return &OfflineModeError{
			Command: cmdName,
		}
	}
	return nil
}

// IsEffectivelyOffline returns true if either CLI flags or config specify offline mode.
func IsEffectivelyOffline() bool {
	if IsOffline() {
		return true
	}
	cfg, err := Effective()
	if err != nil {
		return false
	}
	return cfg.IsOffline()
}

// IsEffectivelyPreferOffline returns true if either CLI flags or config specify prefer-offline mode.
func IsEffectivelyPreferOffline() bool {
	if IsPreferOffline() {
		return true
	}
	cfg, err := Effective()
	if err != nil {
		return false
	}
	return cfg.IsPreferOffline()
}
