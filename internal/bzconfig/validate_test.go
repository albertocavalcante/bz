package bzconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge_RegistryValidation(t *testing.T) {
	for _, reg := range []string{"https://bcr.bazel.build", "http://internal:8080/bcr", "file:///opt/registry", "/opt/registry", `C:\registry`, "D:/registry"} {
		l := parse(t, LayerProject, "/p/.bzconfig.toml", fmt.Sprintf("[network]\nregistry = %q\nfallback_registries = [%q]\n", reg, reg))
		eff, err := Merge(Defaults("/c"), l)
		require.NoError(t, err, reg)
		assert.Equal(t, reg, eff.Network.Registry)
		assert.Equal(t, []string{reg}, eff.Network.FallbackRegistries)
	}

	tests := []struct {
		name, data string
		want       []string
	}{
		{name: "relative registry", data: "[network]\nregistry = \"foo\"\n", want: []string{`"foo"`, "project config /p/.bzconfig.toml"}},
		{name: "registry without host", data: "[network]\nregistry = \"https://\"\n", want: []string{"missing host"}},
		{name: "file URL without path", data: "[network]\nregistry = \"file://\"\n", want: []string{"missing path"}},
		{name: "bad fallback entry", data: "[network]\nfallback_registries = [\"https://ok.example\", \"mirror\"]\n", want: []string{"fallback_registries", `"mirror"`}},
		{name: "ttl in days", data: "[cache]\nttl = \"7d\"\n", want: []string{`invalid cache ttl "7d"`}},
		{name: "negative ttl", data: "[cache]\nttl = \"-1h\"\n", want: []string{`"-1h"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eff, err := Merge(Defaults("/c"), parse(t, LayerProject, "/p/.bzconfig.toml", tt.data))
			require.Error(t, err)
			for _, w := range tt.want {
				assert.Contains(t, err.Error(), w)
			}
			assert.Equal(t, DefaultRegistry, eff.Network.Registry, "invalid values are skipped")
			assert.Empty(t, eff.Network.FallbackRegistries)
			assert.Equal(t, DefaultTTL, eff.Cache.TTL)
		})
	}
}

func TestMerge_EmptyTTLIsAllowed(t *testing.T) {
	eff, err := Merge(Defaults("/c"), parse(t, LayerProject, "/p/.bzconfig.toml", "[cache]\nttl = \"\"\n"))
	require.NoError(t, err)
	assert.Equal(t, LayerProject, eff.Source(KeyCacheTTL).Layer)
}

func TestParseFile_DisabledEntriesAreTrimmed(t *testing.T) {
	l := parse(t, LayerProject, "/p/.bzconfig.toml", "[commands]\ndisabled = [\" audit \", \"\", \"registry ping\"]\n")
	assert.Equal(t, []string{"audit", "registry ping"}, *l.Settings.Disabled)
}
