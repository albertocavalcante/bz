package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/albertocavalcante/bz/internal/cli"
)

func resetFlags() {
	// Reset the global CLI options
	cli.Global = cli.Options{}
	// Reset the flags to their default values
	rootCmd.PersistentFlags().Set("quiet", "false")
	rootCmd.PersistentFlags().Set("no-color", "false")
	rootCmd.PersistentFlags().Set("offline", "false")
	rootCmd.PersistentFlags().Set("prefer-offline", "false")
	rootCmd.PersistentFlags().Set("registry", "")
}

func TestRootCmd_QuietFlag(t *testing.T) {
	resetFlags()

	// Parse the flags directly
	err := rootCmd.PersistentFlags().Parse([]string{"--quiet"})
	assert.NoError(t, err)
	assert.True(t, cli.Global.Quiet, "Quiet flag should be set")
}

func TestRootCmd_QuietFlagShort(t *testing.T) {
	resetFlags()

	err := rootCmd.PersistentFlags().Parse([]string{"-q"})
	assert.NoError(t, err)
	assert.True(t, cli.Global.Quiet, "Quiet flag should be set with -q")
}

func TestRootCmd_NoColorFlag(t *testing.T) {
	resetFlags()

	// Ensure NO_COLOR env is not set for this test
	oldVal, hadVal := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if hadVal {
			os.Setenv("NO_COLOR", oldVal)
		}
	}()

	err := rootCmd.PersistentFlags().Parse([]string{"--no-color"})
	assert.NoError(t, err)
	assert.True(t, cli.Global.NoColor, "NoColor flag should be set")
	assert.False(t, cli.IsColorEnabled(), "Color should be disabled")
}

func TestRootCmd_BothFlags(t *testing.T) {
	resetFlags()

	err := rootCmd.PersistentFlags().Parse([]string{"-q", "--no-color"})
	assert.NoError(t, err)
	assert.True(t, cli.Global.Quiet, "Quiet flag should be set")
	assert.True(t, cli.Global.NoColor, "NoColor flag should be set")
}

func TestRootCmd_FlagsHaveCorrectDefaults(t *testing.T) {
	resetFlags()

	// Verify the default values
	assert.False(t, cli.Global.Quiet, "Quiet should be false by default")
	assert.False(t, cli.Global.NoColor, "NoColor should be false by default")
}

func TestIsColorEnabled_NoColorEnvVar(t *testing.T) {
	// Reset global state
	cli.Global = cli.Options{}

	// Set NO_COLOR env var
	oldVal, hadVal := os.LookupEnv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if hadVal {
			os.Setenv("NO_COLOR", oldVal)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	assert.False(t, cli.IsColorEnabled(), "Color should be disabled when NO_COLOR env var is set")
}

func TestIsColorEnabled_Default(t *testing.T) {
	// Reset global state
	cli.Global = cli.Options{}

	// Ensure NO_COLOR is not set
	oldVal, hadVal := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if hadVal {
			os.Setenv("NO_COLOR", oldVal)
		}
	}()

	assert.True(t, cli.IsColorEnabled(), "Color should be enabled by default")
}

func TestRootCmd_HelpShowsAllCommands(t *testing.T) {
	// This test verifies that all expected commands appear in the help output.
	// The bz CLI should show: init, mod, registry, version commands.

	// Capture help output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()

	// All these commands should appear in the help output
	expectedCommands := []string{
		"init",     // Initialize a new Bazel module project
		"mod",      // Manage Bazel module dependencies
		"registry", // Registry operations
		"tui",      // Launch interactive terminal UI
		"version",  // Print version information
	}

	for _, cmd := range expectedCommands {
		assert.Contains(t, helpOutput, cmd,
			"Help output should contain the '%s' command", cmd)
	}
}

func TestRootCmd_UnknownCommandExitCode(t *testing.T) {
	resetFlags()

	// Create a fresh command for testing to avoid side effects
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"unknowncommand"})

	err := rootCmd.Execute()

	// Unknown command should return an error
	assert.Error(t, err, "Unknown command should return an error")
}

func TestRootCmd_TypoSuggestions(t *testing.T) {
	resetFlags()

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"versoin"}) // typo of "version"

	err := rootCmd.Execute()

	// Should return an error
	assert.Error(t, err)

	// Error message should suggest the correct command
	errOutput := errBuf.String()
	assert.Contains(t, errOutput, "version", "Should suggest 'version' for typo 'versoin'")
}

func TestRootCmd_OfflineFlag(t *testing.T) {
	resetFlags()

	err := rootCmd.PersistentFlags().Parse([]string{"--offline"})
	assert.NoError(t, err)
	assert.True(t, cli.Global.Offline, "Offline flag should be set")
}

func TestRootCmd_PreferOfflineFlag(t *testing.T) {
	resetFlags()

	err := rootCmd.PersistentFlags().Parse([]string{"--prefer-offline"})
	assert.NoError(t, err)
	assert.True(t, cli.Global.PreferOffline, "PreferOffline flag should be set")
}

func TestRootCmd_RegistryFlag(t *testing.T) {
	resetFlags()

	err := rootCmd.PersistentFlags().Parse([]string{"--registry", "https://custom.registry.io"})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.registry.io", cli.Global.Registry, "Registry flag should be set")
}

func TestRootCmd_OfflineAndPreferOfflineMutuallyExclusive(t *testing.T) {
	resetFlags()

	// Set both flags
	err := rootCmd.PersistentFlags().Parse([]string{"--offline", "--prefer-offline"})
	assert.NoError(t, err) // Parsing should succeed

	// But validation should fail
	err = cli.ValidateOfflineFlags()
	assert.Error(t, err, "Should error when both --offline and --prefer-offline are set")
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestRootCmd_OfflineWithBZOfflineEnv(t *testing.T) {
	resetFlags()

	// Set BZ_OFFLINE env var
	oldVal, hadVal := os.LookupEnv("BZ_OFFLINE")
	os.Setenv("BZ_OFFLINE", "1")
	defer func() {
		if hadVal {
			os.Setenv("BZ_OFFLINE", oldVal)
		} else {
			os.Unsetenv("BZ_OFFLINE")
		}
	}()

	// Even without the flag, IsOffline should return true
	assert.True(t, cli.IsOffline(), "IsOffline should return true when BZ_OFFLINE env var is set")
}

func TestRootCmd_NewFlagsHaveCorrectDefaults(t *testing.T) {
	resetFlags()

	// Verify the default values for new flags
	assert.False(t, cli.Global.Offline, "Offline should be false by default")
	assert.False(t, cli.Global.PreferOffline, "PreferOffline should be false by default")
	assert.Empty(t, cli.Global.Registry, "Registry should be empty by default")
}

func TestRootCmd_HelpShowsOfflineFlags(t *testing.T) {
	resetFlags()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	assert.NoError(t, err)

	helpOutput := buf.String()

	// All new flags should appear in the help output
	assert.Contains(t, helpOutput, "--offline", "Help should mention --offline flag")
	assert.Contains(t, helpOutput, "--prefer-offline", "Help should mention --prefer-offline flag")
	assert.Contains(t, helpOutput, "--registry", "Help should mention --registry flag")
	assert.Contains(t, helpOutput, "air-gapped", "Help should mention air-gapped usage")
}
