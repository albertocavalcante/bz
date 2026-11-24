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

func TestMerge_RegistryIsStoredTrimmed(t *testing.T) {
	for _, raw := range []string{"  https://x.example  ", "https://x.example\n", "\thttps://x.example"} {
		reg := raw
		fallback := []string{raw, " /opt/registry "}
		eff, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: "/p", Settings: Settings{Registry: &reg, FallbackRegistries: &fallback}})
		require.NoError(t, err, "%q", raw)
		assert.Equal(t, "https://x.example", eff.Network.Registry)
		assert.Equal(t, []string{"https://x.example", "/opt/registry"}, eff.Network.FallbackRegistries)
	}
}

func TestMerge_RegistryRejectsHostlessAndControlCharacters(t *testing.T) {
	for _, raw := range []string{"http://:80", "https://:443/bcr", "https://x.example/a\nb", "/opt/re\x1bgistry"} {
		reg := raw
		eff, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: "/p", Settings: Settings{Registry: &reg}})
		require.Error(t, err, "%q", raw)
		assert.Equal(t, DefaultRegistry, eff.Network.Registry)
		assertNoControlBytes(t, err.Error())
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

func TestRedactURL(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"no credentials", "https://bcr.bazel.build/x", "https://bcr.bazel.build/x"},
		{"user and password", "https://user:s3cret@host.example/p", "https://xxxxx@host.example/p"},
		{"token only", "https://tok3n@host.example", "https://xxxxx@host.example"},
		{"slash in password", "https://user:pa/ss@host", "https://xxxxx@host"},
		{"at signs in userinfo", "https://a@b:c@host/x", "https://xxxxx@host/x"},
		{"no scheme", "user:s3cret@host", "xxxxx@host"},
		{"no scheme with port", "s3cret@host:80", "xxxxx@host:80"},
		{"at in query is kept", "https://host/p?u=a@b", "https://host/p?u=a@b"},
		{"at in fragment is kept", "https://host/p#a@b", "https://host/p#a@b"},
		{"path", "/opt/registry", "/opt/registry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RedactURL(tt.in))
		})
	}
}

// TestMerge_RegistryCredentialsNeverPrinted: credentials are accepted and
// kept for use, but no error shows them.
func TestMerge_RegistryCredentialsNeverPrinted(t *testing.T) {
	good := "https://user:s3cret@registry.example/bcr"
	eff, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: "/p", Settings: Settings{Registry: &good}})
	require.NoError(t, err)
	assert.Equal(t, good, eff.Network.Registry)

	const prefix = "invalid registry in project config /p: "
	tests := []struct{ name, in, want string }{
		{"missing host", "https://user:s3cret@", prefix + `"https://xxxxx@": missing host`},
		{"bad escape", "https://user:s3cret@host/%zz", prefix + `"https://xxxxx@host/%zz": not a valid URL`},
		{"slash in password", "https://user:pa/ss@host", prefix + `"https://xxxxx@host": not a valid URL`},
		{"bad port", "https://user:s3cret@host:pa", prefix + `"https://xxxxx@host:pa": not a valid URL`},
		{"unsupported scheme", "ftp://user:s3cret@host/registry", prefix + `"ftp://xxxxx@host/registry": want an https://, http:// or file:// URL or an absolute path`},
		{"no scheme", "user:s3cret@host", prefix + `"xxxxx@host": want an https://, http:// or file:// URL or an absolute path`},
		{"no scheme with port", "s3cret@host:80", prefix + `"xxxxx@host:80": want an https://, http:// or file:// URL or an absolute path`},
		{"control character", "https://user:s3cret@host/a\nb", prefix + `"https://xxxxx@host/a\nb": contains control or non-printable characters`},
		{"token without host name", "https://s3cret@:443/bcr", prefix + `"https://xxxxx@:443/bcr": missing host`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := tt.in
			_, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: "/p", Settings: Settings{Registry: &reg}})
			require.EqualError(t, err, tt.want)
		})
	}
}
