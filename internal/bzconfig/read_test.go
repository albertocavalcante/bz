package bzconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessagesRenderHostilePaths(t *testing.T) {
	const bad = "/w\nz\x1b[31m\xff/.bz.toml"
	const shown = `"/w\nz\x1b[31m\xff/.bz.toml"`

	assert.Equal(t, "project config "+shown, Source{Layer: LayerProject, Origin: bad}.String())

	_, err := ParseFile(LayerProject, bad, []byte("[network\n"), "")
	require.EqualError(t, err, "parsing project config "+shown+": invalid TOML at line 1, column 9: invalid table name")

	m := NetworkMode("bogus")
	_, err = Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: bad, Settings: Settings{Mode: &m}})
	require.EqualError(t, err, `invalid network mode "bogus" in project config `+shown+` (want "online", "prefer-offline" or "offline")`)
}
