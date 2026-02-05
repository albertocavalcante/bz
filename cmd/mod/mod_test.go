package mod

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModCmd_TypoSuggestions(t *testing.T) {
	// Reset output
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	Cmd.SetOut(buf)
	Cmd.SetErr(errBuf)
	Cmd.SetArgs([]string{"serach"}) // typo of "search"

	err := Cmd.Execute()

	// Should return an error for unknown subcommand
	assert.Error(t, err)

	// Error message should suggest the correct command
	assert.Contains(t, err.Error(), "search", "Should suggest 'search' for typo 'serach'")
	assert.Contains(t, err.Error(), "Did you mean", "Should show suggestion prompt")
}

func TestModCmd_UnknownSubcommandError(t *testing.T) {
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	Cmd.SetOut(buf)
	Cmd.SetErr(errBuf)
	Cmd.SetArgs([]string{"unknownsubcmd"})

	err := Cmd.Execute()

	// Unknown subcommand should return an error
	assert.Error(t, err, "Unknown subcommand should return an error")
}
