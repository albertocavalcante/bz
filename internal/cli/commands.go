package cli

import (
	"fmt"
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

// effectiveOrFlags returns the effective configuration, or, when it cannot
// be computed (only reachable when a command is invoked without the root
// pre-run, which reports the error), defaults plus the flag layer.
func effectiveOrFlags() *bzconfig.Effective {
	if eff, err := Effective(); err == nil {
		return eff
	}
	// Merge skips invalid values (an invalid --registry) and still returns
	// a usable configuration.
	eff, _ := bzconfig.Merge(bzconfig.Defaults(bzconfig.DefaultCacheDir(configEnv)), flagLayer())
	return eff
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

// CheckCommandEnabled enforces [commands] disabled for the command at path
// (its command path without the root, e.g. "registry ping"). commands lists
// every command path in the tree so entries can be resolved and checked.
// It returns warnings for entries that match nothing, and an error when the
// command is disabled or an entry is ambiguous. Protected commands (help,
// version, completion) are never disabled.
func CheckCommandEnabled(eff *bzconfig.Effective, path string, commands []string) ([]string, error) {
	source := eff.Source(bzconfig.KeyDisabled)
	set, warnings, err := bzconfig.ResolveDisabled(eff.Commands.Disabled, commands, source)
	if err != nil {
		return warnings, err
	}
	if bzconfig.IsProtectedCommand(path) {
		return warnings, nil
	}
	if entry, ok := set.Match(path); ok {
		return warnings, &CommandDisabledError{
			Command: path,
			Reason:  fmt.Sprintf("listed as %q in [commands] disabled (%s)", entry, source),
		}
	}
	return warnings, nil
}

// OfflineModeError is returned when a command requires network access but offline mode is enabled.
type OfflineModeError struct {
	Command     string
	Description string
	// Source says which layer selected offline mode, e.g. "flag --offline".
	Source string
}

func (e *OfflineModeError) Error() string {
	msg := fmt.Sprintf("'bz %s' requires network access", e.Command)
	if e.Description != "" {
		msg += " " + e.Description
	}
	msg += ".\n\nIn offline mode, this operation is not available."
	if e.Source != "" {
		msg += fmt.Sprintf(" Offline mode is set by %s.", e.Source)
	}
	msg += "\n\nOptions:\n"
	msg += "  1. Run with network access (remove --offline, BZ_OFFLINE, or [network] mode = \"offline\" from your bz config file)\n"
	msg += fmt.Sprintf("  2. Disable this command in .bz.toml:\n     [commands]\n     disabled = [%q]\n", e.Command)
	return msg
}

// Is implements errors.Is for OfflineModeError.
func (e *OfflineModeError) Is(target error) bool {
	_, ok := target.(*OfflineModeError)
	return ok
}

// CheckOfflineAllowed returns OfflineModeError when the effective network
// mode is offline, and the load error when the configuration is invalid.
func CheckOfflineAllowed(cmdName string) error {
	eff, err := Effective()
	if err != nil {
		return err // fail closed: never run a network command on a broken config
	}
	if eff.IsOffline() {
		return &OfflineModeError{
			Command: cmdName,
			Source:  eff.Source(bzconfig.KeyMode).String(),
		}
	}
	return nil
}
