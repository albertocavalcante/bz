package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestOfflineRegistryCommandsDoNotSendRequests(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	for _, command := range []struct {
		name string
		args func(*testing.T) []string
	}{
		{"registry ping", func(_ *testing.T) []string { return []string{"registry", "ping", server.URL} }},
		{"cache download", func(t *testing.T) []string {
			return []string{"cache", "download", "--registry", server.URL, "--cache-dir", filepath.Join(t.TempDir(), "cache"), "fixture@1.0.0"}
		}},
		{"sbom", func(_ *testing.T) []string { return []string{"--registry", server.URL, "sbom"} }},
		{"mod info", func(_ *testing.T) []string { return []string{"mod", "info", "fixture", "--registry", server.URL} }},
		{"mod sync HTTP origin", func(t *testing.T) []string {
			config, _ := writeSyncConfig(t, server.URL, `"fixture"`, "")
			content, err := os.ReadFile(config)
			require.NoError(t, err)
			// The published guide uses an HTTP origin.
			updated := strings.Replace(string(content), "bcr = registry.file(", "bcr = registry.http(", 1)
			require.NoError(t, os.WriteFile(config, []byte(updated), 0o644))
			return []string{"mod", "sync", "-c", config, "mirror"}
		}},
		{"mod sync HTTP destination", func(t *testing.T) []string {
			origin := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
				"fixture": {Versions: []string{"1.0.0"}},
			})
			config, dest := writeSyncConfig(t, origin, `"fixture"`, "")
			content, err := os.ReadFile(config)
			require.NoError(t, err)
			updated := strings.Replace(string(content), "mirror = registry.file("+quote(dest)+")",
				"mirror = registry.http_put("+quote(server.URL)+")", 1)
			require.NotEqual(t, string(content), updated)
			require.NoError(t, os.WriteFile(config, []byte(updated), 0o644))
			return []string{"mod", "sync", "-c", config, "mirror"}
		}},
	} {
		for _, mode := range []string{"flag", "environment", "project config"} {
			t.Run(command.name+"/"+mode, func(t *testing.T) {
				syncEnv(t)
				writeOfflineTestModule(t)
				args := command.args(t)
				switch mode {
				case "flag":
					args = append([]string{"--offline"}, args...)
				case "environment":
					t.Setenv("BZ_OFFLINE", "1")
				case "project config":
					require.NoError(t, os.WriteFile(".bzconfig.toml", []byte("[network]\nmode = \"offline\"\n"), 0o644))
				}
				before := requests.Load()
				stdout, stderr, code := runCLI(t, args...)
				assert.Equal(t, before, requests.Load(), "offline CLI contacted the HTTP registry")
				assert.NotEqual(t, 0, code, "stdout=%q", stdout)
				assert.Contains(t, stderr, "offline")
			})
		}
	}
}

type offlineTestTransport struct {
	requests atomic.Int64
}

func (transport *offlineTestTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	transport.requests.Add(1)
	return nil, os.ErrPermission
}

func TestOfflineRegistryCommandsDefaultHost(t *testing.T) {
	syncEnv(t)
	writeOfflineTestModule(t)
	transport := &offlineTestTransport{}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })

	for _, args := range [][]string{
		{"--offline", "registry", "ping"},
		{"--offline", "cache", "download", "--cache-dir", t.TempDir(), "fixture@1.0.0"},
		{"--offline", "sbom"},
		{"--offline", "mod", "info", "fixture"},
	} {
		stdout, stderr, code := runCLI(t, args...)
		assert.NotEqual(t, 0, code, "args=%v stdout=%q", args, stdout)
		assert.Contains(t, stderr, "offline", "args=%v", args)
	}
	assert.Zero(t, transport.requests.Load(), "offline CLI attempted HTTP to the default registry")
}

func TestOfflineLocalRegistryCommands(t *testing.T) {
	syncEnv(t)
	writeOfflineTestModule(t)
	origin := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"fixture": {Versions: []string{"1.0.0"}},
	})
	config, _ := writeSyncConfig(t, origin, `"fixture"`, "")
	for _, args := range [][]string{
		{"--offline", "--registry", origin, "sbom"},
		{"--offline", "mod", "info", "fixture", "--registry", origin},
		{"--offline", "cache", "download", "--registry", origin, "--cache-dir", t.TempDir(), "fixture@1.0.0"},
		{"--offline", "mod", "sync", "-c", config, "mirror"},
		{"--offline", "mod", "sync", "-c", config, "--list"},
		{"--offline", "sbom", "--include-transitive=false"},
	} {
		stdout, stderr, code := runCLI(t, args...)
		assert.Equal(t, 0, code, "args=%v stdout=%q stderr=%q", args, stdout, stderr)
	}
}

func TestOfflineCacheWithoutDependencies(t *testing.T) {
	syncEnv(t)
	require.NoError(t, os.WriteFile("MODULE.bazel", []byte("module(name = \"empty\")\n"), 0o644))
	stdout, stderr, code := runCLI(t, "--offline", "cache", "download")
	assert.Equal(t, 0, code, "stderr=%q", stderr)
	assert.Contains(t, stdout, "No dependencies found in MODULE.bazel")
}

func writeOfflineTestModule(t *testing.T) {
	t.Helper()
	const content = "module(name = \"example\")\nbazel_dep(name = \"fixture\", version = \"1.0.0\")\n"
	require.NoError(t, os.WriteFile("MODULE.bazel", []byte(content), 0o644))
}
