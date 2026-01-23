package mod

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfoCmd_RequiresModuleArg(t *testing.T) {
	err := runInfo(infoCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "module argument required")
}

func TestInfoCmd_ShowsMetadataWithoutVersion(t *testing.T) {
	// This is an integration test - skip in CI if needed
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	infoJSON = false

	var stdout bytes.Buffer
	infoCmd.SetOut(&stdout)

	// Without version, should show metadata
	err := runInfo(infoCmd, []string{"rules_go"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "Latest:")
	assert.Contains(t, output, "Versions:")
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
	err := runInfo(infoCmd, []string{"rules_go@0.50.1"})
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

	err := runInfo(infoCmd, []string{"rules_go@0.50.1"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, `"name"`)
	assert.Contains(t, output, `"dependencies"`)
}
