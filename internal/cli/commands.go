package cli

import (
	"fmt"
	"slices"
	"sync"

	"github.com/albertocavalcante/bz/internal/bzconfig"
)

var (
	cachedCfg     *bzconfig.Effective
	cachedCfgOnce sync.Once
	cachedCfgErr  error
)

func loadCachedConfig() (*bzconfig.Effective, error) {
	cachedCfgOnce.Do(func() {
		cachedCfg, cachedCfgErr = bzconfig.LoadProjectFile(".bzconfig.toml")
	})
	return cachedCfg, cachedCfgErr
}

// ResetCachedConfig resets the cached configuration so it will be reloaded on next access.
// This is intended for use in tests.
func ResetCachedConfig() {
	cachedCfgOnce = sync.Once{}
	cachedCfg = nil
	cachedCfgErr = nil
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
	msg += " and cannot run in offline mode.\n"
	msg += "To allow network access, remove --offline, unset BZ_OFFLINE, or check [network].mode in .bzconfig.toml."
	return msg
}

// Is implements errors.Is for OfflineModeError.
func (e *OfflineModeError) Is(target error) bool {
	_, ok := target.(*OfflineModeError)
	return ok
}

// CheckCommandAllowed checks if a command is allowed based on configuration.
// Returns CommandDisabledError if the command is disabled.
func CheckCommandAllowed(cmdName string) error {
	cfg, err := loadCachedConfig()
	if err != nil {
		return nil //nolint:nilerr // config loading failure is non-fatal; allow command to proceed
	}

	if slices.Contains(cfg.Commands.Disabled, cmdName) {
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
	cfg, err := loadCachedConfig()
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

	cfg, err := loadCachedConfig()
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

	cfg, err := loadCachedConfig()
	if err != nil {
		return false
	}

	return cfg.IsPreferOffline()
}
