package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/testutil"
)

func modInfoRegistry(t *testing.T) string {
	t.Helper()
	return testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.49.0", "0.50.1"}},
	})
}

// TestRun_ModInfoUsesConfiguredRegistry proves mod info builds its registry
// from the config files (a file registry fixture; never the network).
func TestRun_ModInfoUsesConfiguredRegistry(t *testing.T) {
	reg := modInfoRegistry(t)
	regLine := "[network]\nregistry = \"" + filepath.ToSlash(reg) + "\"\n"

	t.Run("project file", func(t *testing.T) {
		configWorkspace(t, regLine, "")
		stdout, stderr, code := runCLI(t, "mod", "info", "rules_go")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.Contains(t, stdout, "0.50.1")
	})

	t.Run("user file", func(t *testing.T) {
		configWorkspace(t, "", regLine)
		stdout, stderr, code := runCLI(t, "mod", "info", "rules_go")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.Contains(t, stdout, "0.50.1")
	})

	t.Run("user offline mode applies", func(t *testing.T) {
		configWorkspace(t, "", regLine+"mode = \"offline\"\n")
		stdout, stderr, code := runCLI(t, "mod", "info", "rules_go")
		assert.Equal(t, 1, code)
		assert.NotContains(t, stdout, "0.50.1")
		assert.Contains(t, strings.ToLower(stderr), "offline")
	})

	t.Run("--prefer-offline beats config offline", func(t *testing.T) {
		configWorkspace(t, "", regLine+"mode = \"offline\"\n")
		stdout, stderr, code := runCLI(t, "--prefer-offline", "mod", "info", "rules_go")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.Contains(t, stdout, "0.50.1")
	})

	t.Run("project mode=online resets user offline", func(t *testing.T) {
		configWorkspace(t, "[network]\nmode = \"online\"\n", regLine+"mode = \"offline\"\n")
		stdout, stderr, code := runCLI(t, "mod", "info", "rules_go")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.Contains(t, stdout, "0.50.1")
	})

	t.Run("--registry flag beats config", func(t *testing.T) {
		configWorkspace(t, "[network]\nregistry = \"/nonexistent/registry\"\n", "")
		stdout, stderr, code := runCLI(t, "--registry", reg, "mod", "info", "rules_go")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.Contains(t, stdout, "0.50.1")
	})
}

func TestRun_CacheDirFromEnvAndConfig(t *testing.T) {
	statsDir := func(t *testing.T, args ...string) string {
		t.Helper()
		stdout, stderr, code := runCLI(t, append([]string{"cache", "stats", "--json"}, args...)...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var out struct {
			CacheDir string `json:"cache_dir"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
		return out.CacheDir
	}

	t.Run("config", func(t *testing.T) {
		configWorkspace(t, "[cache]\ndir = \"/from/config\"\n", "")
		assert.Equal(t, "/from/config", statsDir(t))
	})
	t.Run("BZ_CACHE_DIR beats config", func(t *testing.T) {
		configWorkspace(t, "[cache]\ndir = \"/from/config\"\n", "")
		t.Setenv("BZ_CACHE_DIR", "/from/env")
		assert.Equal(t, "/from/env", statsDir(t))
	})
	t.Run("--cache-dir beats BZ_CACHE_DIR", func(t *testing.T) {
		configWorkspace(t, "", "")
		t.Setenv("BZ_CACHE_DIR", "/from/env")
		assert.Equal(t, "/from/flag", statsDir(t, "--cache-dir", "/from/flag"))
	})
}

func TestDoctorRegistryCheck_UsesEffectiveRegistry(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = "http://127.0.0.1:1" // must not be used
	t.Cleanup(func() { defaultDoctorRegistryURL = oldURL })
	resetDoctorFlags()

	t.Run("config registry", func(t *testing.T) {
		configWorkspace(t, "[network]\nregistry = \""+server.URL+"\"\n", "")
		cli.ResetCachedConfig()
		check := checkRegistryConnectivity(context.Background(), io.Discard)
		assert.Equal(t, statusPass, check.Status, check.Message)
		assert.Equal(t, 1, hits)
	})

	t.Run("local registry is not pinged", func(t *testing.T) {
		configWorkspace(t, "[network]\nregistry = \"/srv/registry\"\n", "")
		cli.ResetCachedConfig()
		check := checkRegistryConnectivity(context.Background(), io.Discard)
		assert.Equal(t, statusSkipped, check.Status)
		assert.Contains(t, check.Message, "/srv/registry")
	})

	t.Run("config offline skips", func(t *testing.T) {
		configWorkspace(t, "", "[network]\nmode = \"offline\"\n")
		cli.ResetCachedConfig()
		check := checkRegistryConnectivity(context.Background(), io.Discard)
		assert.Equal(t, statusSkipped, check.Status)
		assert.Contains(t, check.Message, "offline")
	})
}
