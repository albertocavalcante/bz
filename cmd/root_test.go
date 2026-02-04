package cmd

import (
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
