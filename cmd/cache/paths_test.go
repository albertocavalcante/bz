package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/bzconfig"
	"github.com/albertocavalcante/bz/internal/cli"
)

// TestResolveCacheDir covers --cache-dir > BZ_CACHE_DIR > [cache] dir >
// platform default.
func TestResolveCacheDir(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		env      string
		config   string
		want     string
		wantHome string // relative to the fake home, when set
	}{
		{name: "flag wins", flag: "/flag", env: "/env", config: "/config", want: "/flag"},
		{name: "env beats config", env: "/env", config: "/config", want: "/env"},
		{name: "config", config: "/config", want: "/config"},
		{name: "config with tilde", config: "~/bzcache", wantHome: "bzcache"},
		{name: "default", wantHome: filepath.Join(".cache", "bz")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			ws := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(ws, "MODULE.bazel"), nil, 0o644))
			if tt.config != "" {
				require.NoError(t, os.WriteFile(filepath.Join(ws, ".bz.toml"),
					[]byte("[cache]\ndir = \""+tt.config+"\"\n"), 0o644))
			}
			env := bzconfig.OSEnvironment().Isolated(t.TempDir()).WithEnv(map[string]string{
				"BZ_CACHE_DIR": tt.env, "XDG_CACHE_HOME": "",
			})
			env.UserHomeDir = func() (string, error) { return home, nil }
			t.Cleanup(cli.SetEnvironment(env))
			t.Chdir(ws)

			got, err := resolveCacheDir(tt.flag)
			require.NoError(t, err)
			want := tt.want
			if tt.wantHome != "" {
				want = filepath.Join(home, tt.wantHome)
			}
			assert.Equal(t, want, got)
		})
	}
}
