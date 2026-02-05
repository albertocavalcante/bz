package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd_PlainOutput(t *testing.T) {
	versionJSON = false

	var stdout bytes.Buffer
	versionCmd.SetOut(&stdout)

	versionCmd.Run(versionCmd, []string{})

	output := stdout.String()
	assert.Contains(t, output, "bz")
	assert.Contains(t, output, version)
}

func TestVersionCmd_JSONOutput(t *testing.T) {
	versionJSON = true
	defer func() { versionJSON = false }()

	var stdout bytes.Buffer
	versionCmd.SetOut(&stdout)

	versionCmd.Run(versionCmd, []string{})

	output := stdout.String()

	// Verify it's valid JSON
	var result map[string]interface{}
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Check expected fields
	assert.Equal(t, version, result["version"])
	assert.Equal(t, commit, result["commit"])
	assert.Equal(t, date, result["build_date"])
}

func TestVersionCmd_JSONOutputFormat(t *testing.T) {
	versionJSON = true
	defer func() { versionJSON = false }()

	var stdout bytes.Buffer
	versionCmd.SetOut(&stdout)

	versionCmd.Run(versionCmd, []string{})

	output := stdout.String()

	// Verify expected JSON keys exist
	assert.Contains(t, output, `"version"`)
	assert.Contains(t, output, `"commit"`)
	assert.Contains(t, output, `"build_date"`)
}

func TestVersionCmd_ShortFlag(t *testing.T) {
	// Test that -v flag works as an alias for the version command
	// This tests the root command's -v flag, not the version subcommand
	versionJSON = false

	// Test by directly checking flag parsing and run behavior
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)

	// Parse the -v flag
	err := rootCmd.Flags().Parse([]string{"-v"})
	require.NoError(t, err)

	// Check that showVersion is now true
	require.True(t, showVersion, "-v flag should set showVersion to true")

	// Manually call the Run function to simulate execution
	rootCmd.Run(rootCmd, []string{})

	// Reset for other tests
	showVersion = false
	rootCmd.Flags().Set("version", "false")

	output := stdout.String()
	assert.Contains(t, output, "bz")
	assert.Contains(t, output, version)
	assert.Contains(t, output, "built on")
}
