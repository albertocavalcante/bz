package bzconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessagesRenderHostilePaths(t *testing.T) {
	const bad = "/w\nz\x1b[31m\xff/.bz.toml"
	s := Source{Layer: LayerProject, Origin: bad}
	assertNoControlBytes(t, s.String())

	_, err := ParseFile(LayerProject, bad, []byte("[network\n"), "")
	require.Error(t, err)
	assertNoControlBytes(t, err.Error())

	m := NetworkMode("bogus")
	_, err = Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: bad, Settings: Settings{Mode: &m}})
	require.Error(t, err)
	assertNoControlBytes(t, err.Error())
}
