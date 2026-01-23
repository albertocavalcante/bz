package mod

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfoCmd_RequiresModuleArg(t *testing.T) {
	err := infoCmd.RunE(infoCmd, []string{})
	require.Error(t, err)
}

func TestInfoCmd_RequiresVersion(t *testing.T) {
	// Reset flags
	infoJSON = false

	var stderr bytes.Buffer
	infoCmd.SetErr(&stderr)

	err := infoCmd.RunE(infoCmd, []string{"rules_go"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestInfoCmd_FetchesFromRegistry(t *testing.T) {
	// This is an integration test - skip in CI if needed
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	infoJSON = false

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	// Use a real module that exists in BCR
	err := infoCmd.RunE(infoCmd, []string{"rules_go@0.50.1"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.50.1")
}

func TestInfoCmd_JSONOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	infoJSON = true
	defer func() { infoJSON = false }()

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	err := infoCmd.RunE(infoCmd, []string{"rules_go@0.50.1"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, `"name"`)
	assert.Contains(t, output, `"dependencies"`)
}
