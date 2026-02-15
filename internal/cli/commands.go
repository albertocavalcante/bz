package cli

import (
	"errors"
	"fmt"

	"github.com/albertocavalcante/bz/internal/bzconfig"
)

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
	var t *CommandDisabledError
	return errors.As(target, &t)
}

// OfflineModeError is returned when a command requires network access but offline mode is enabled.
type OfflineModeError struct {
	Command     string
	Description string
}

func (e *OfflineModeError) Error() string {
	msg := fmt.Sprintf("Error: 'bz %s' requires network access", e.Command)
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
	var t *OfflineModeError
	return errors.As(target, &t)
}

// CheckCommandAllowed checks if a command is allowed based on configuration.
// Returns CommandDisabledError if the command is disabled.
func CheckCommandAllowed(cmdName string) error {
	cfg, err := bzconfig.Load(nil, bzconfig.WithProjectConfig(".bzconfig.toml"))
	if err != nil {
		return nil //nolint:nilerr // config loading failure is non-fatal; allow command to proceed
	}

	if cfg.IsCommandDisabled(cmdName) {
		return &CommandDisabledError{
			Command: cmdName,
			Reason:  "disabled in configuration",
		}
	}

	return nil
}

// CheckOfflineAllowed checks if network access is available for a command.
// Returns OfflineModeError if offline mode is enabled.
// This checks both CLI flags and configuration.
func CheckOfflineAllowed(cmdName string) error {
	// Check CLI flag first
	if IsOffline() {
		return &OfflineModeError{
			Command: cmdName,
		}
	}

	// Also check config file
	cfg, err := bzconfig.Load(nil, bzconfig.WithProjectConfig(".bzconfig.toml"))
	if err != nil {
		return nil //nolint:nilerr // config loading failure is non-fatal; allow command to proceed
	}

	if cfg.IsOffline() {
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

	cfg, err := bzconfig.Load(nil, bzconfig.WithProjectConfig(".bzconfig.toml"))
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

	cfg, err := bzconfig.Load(nil, bzconfig.WithProjectConfig(".bzconfig.toml"))
	if err != nil {
		return false
	}

	return cfg.IsPreferOffline()
}
