package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionCmd_Bash(t *testing.T) {
	var stdout bytes.Buffer
	completionCmd.SetOut(&stdout)

	err := completionCmd.RunE(completionCmd, []string{"bash"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "bash completion", "should contain bash completion header")
	assert.Contains(t, output, "bz", "should reference bz command")
}

func TestCompletionCmd_Zsh(t *testing.T) {
	var stdout bytes.Buffer
	completionCmd.SetOut(&stdout)

	err := completionCmd.RunE(completionCmd, []string{"zsh"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "zsh", "should contain zsh reference")
}

func TestCompletionCmd_Fish(t *testing.T) {
	var stdout bytes.Buffer
	completionCmd.SetOut(&stdout)

	err := completionCmd.RunE(completionCmd, []string{"fish"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "fish", "should contain fish reference")
}

func TestCompletionCmd_PowerShell(t *testing.T) {
	var stdout bytes.Buffer
	completionCmd.SetOut(&stdout)

	err := completionCmd.RunE(completionCmd, []string{"powershell"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "bz", "should reference bz command")
}

func TestCompletionCmd_InvalidShell(t *testing.T) {
	err := completionCmd.RunE(completionCmd, []string{"invalid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestCompletionCmd_ValidArgsSet(t *testing.T) {
	// Verify the command is configured correctly
	assert.ElementsMatch(t, []string{"bash", "zsh", "fish", "powershell"}, completionCmd.ValidArgs)
}
