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
	const prefix = "invalid registry in project config /p: "
	const help = "\nhelp: want https://host[:port]/path, file:///abs/path or an absolute path"
	tests := []struct{ name, in, want string }{
		{"port without host", "http://:80", prefix + `"http://:80": missing host` + help},
		{"port and path without host", "https://:443/bcr", prefix + `"https://:443/bcr": missing host` + help},
		{"newline", "https://x.example/a\nb", prefix + `"https://x.example/a\nb": contains control or non-printable characters`},
		{"escape", "/opt/re\x1bgistry", prefix + `"/opt/re\x1bgistry": contains control or non-printable characters`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := tt.in
			eff, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: "/p", Settings: Settings{Registry: &reg}})
			require.EqualError(t, err, tt.want)
			assert.Equal(t, DefaultRegistry, eff.Network.Registry)
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

func TestRedactURL(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"no credentials", "https://bcr.bazel.build/x", "https://bcr.bazel.build/x"},
		{"user and password", "https://user:s3cret@host.example/p", "https://xxxxx@host.example/p"},
		{"token only", "https://tok3n@host.example", "https://xxxxx@host.example"},
		{"escaped at in password", "https://user:pa%40ss@host/p", "https://xxxxx@host/p"},
		{"at in path is kept", "https://host/x@y", "https://host/x@y"},
		{"at in query is kept", "https://host/p?u=a@b", "https://host/p?u=a@b"},
		{"question mark in password", "https://user:pa?ss@host", "https://<redacted>"},
		{"hash in password", "https://user:pa#ss@host", "https://<redacted>"},
		{"slash in password", "https://user:pa/ss@host", "https://<redacted>"},
		{"bad port after credentials", "https://user:s3cret@host:bad", "https://<redacted>"},
		{"ambiguous path or password", "https://host:bad/x@y", "https://<redacted>"},
		{"no scheme", "user:s3cret@host", "<redacted>"},
		{"no scheme with port", "s3cret@host:80", "<redacted>"},
		{"path", "/opt/registry", "/opt/registry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RedactURL(tt.in))
		})
	}
}

// TestMerge_RegistryErrorMessages: each failure class has a fixed message
// that never shows credentials; malformed URLs end with a help line.
func TestMerge_RegistryErrorMessages(t *testing.T) {
	good := "https://user:s3cret@registry.example/bcr"
	eff, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: "/p", Settings: Settings{Registry: &good}})
	require.NoError(t, err)
	assert.Equal(t, good, eff.Network.Registry)

	const prefix = "invalid registry in project config /p: "
	const help = "\nhelp: want https://host[:port]/path, file:///abs/path or an absolute path"
	tests := []struct{ name, in, want string }{
		{"missing host", "https://user:s3cret@", prefix + `"https://<redacted>": missing host` + help},
		{"missing host without credentials", "http://:80", prefix + `"http://:80": missing host` + help},
		{"bad percent-escape", "https://user:s3cret@host/%zz", prefix + `"https://<redacted>": bad percent-escape` + help},
		{"invalid port", "https://user:s3cret@host:pa", prefix + `"https://<redacted>": invalid port` + help},
		{"question mark in password", "https://user:pa?ss@host", prefix + `"https://<redacted>": invalid port` + help},
		{"hash in password", "https://user:pa#ss@host", prefix + `"https://<redacted>": invalid port` + help},
		{"invalid character in host", "https://user:s3cret@ho^st/", prefix + `"https://<redacted>": invalid character in host` + help},
		{"words in the value do not pick the category", "https://[::1/invalid port", prefix + `"https://[::1/invalid port": not a valid URL` + help},
		{"credentials and @ in the path", "ftp://user:s3cret@host/@scope/reg", prefix + `"ftp://xxxxx@host/@scope/reg": want an https://, http:// or file:// URL or an absolute path`},
		{"unsupported scheme", "ftp://user:s3cret@host/registry", prefix + `"ftp://xxxxx@host/registry": want an https://, http:// or file:// URL or an absolute path`},
		{"no scheme", "user:s3cret@host", prefix + `"<redacted>": want an https://, http:// or file:// URL or an absolute path`},
		{"control character", "https://user:s3cret@host/a\nb", prefix + `"https://<redacted>": contains control or non-printable characters`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := tt.in
			_, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: "/p", Settings: Settings{Registry: &reg}})
			require.EqualError(t, err, tt.want)
		})
	}
}

func TestMerge_FallbackAndEnvRegistryMessages(t *testing.T) {
	fallback := []string{"https://ok.example", "https://user:s3cret@host:bad"}
	_, err := Merge(Defaults("/c"), Layer{Kind: LayerProject, Origin: "/p", Settings: Settings{FallbackRegistries: &fallback}})
	require.EqualError(t, err, `invalid fallback_registries entry in project config /p: "https://<redacted>": invalid port`+
		"\nhelp: want https://host[:port]/path, file:///abs/path or an absolute path")

	env, err := EnvLayer(lookupFrom(map[string]string{EnvRegistry: "s3cret@host:80"}), "")
	require.NoError(t, err)
	_, err = Merge(Defaults("/c"), env)
	require.EqualError(t, err, `invalid registry in environment BZ_REGISTRY: "<redacted>": want an https://, http:// or file:// URL or an absolute path`)
}
