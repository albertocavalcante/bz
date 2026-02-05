package cache

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheCmd_NoSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	Cmd.SetOut(&stdout)
	Cmd.SetArgs([]string{})

	err := Cmd.Execute()
	assert.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "cache")
	assert.Contains(t, output, "Commands for managing the local cache")
}

func TestCacheCmd_Help(t *testing.T) {
	var stdout bytes.Buffer
	Cmd.SetOut(&stdout)
	Cmd.SetArgs([]string{"--help"})

	err := Cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "cache")
	// Verify subcommands are registered
	assert.Contains(t, output, "download")
	assert.Contains(t, output, "stats")
	assert.Contains(t, output, "verify")
	assert.Contains(t, output, "clear")
}

func TestCacheCmd_UnknownSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	Cmd.SetOut(&stdout)
	Cmd.SetErr(&stderr)
	Cmd.SetArgs([]string{"notacommand"})

	err := Cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}
