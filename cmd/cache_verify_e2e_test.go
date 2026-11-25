package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestRun_CacheVerifyReportsMissingTransitiveModule(t *testing.T) {
	syncEnv(t)
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"parent": {
			Versions: []string{"1.0.0"},
			Deps:     map[string][]string{"1.0.0": {"child@2.0.0"}},
		},
		"child": {Versions: []string{"2.0.0"}},
	})
	cacheDir := t.TempDir()
	require.NoError(t, os.WriteFile("MODULE.bazel", []byte(`module(name = "project", version = "1.0.0")
bazel_dep(name = "parent", version = "1.0.0")
`), 0o644))

	stdout, stderr, code := runCLI(t, "cache", "download", "--registry="+registryDir, "--cache-dir="+cacheDir)
	require.Equalf(t, 0, code, "stdout=%s\nstderr=%s", stdout, stderr)

	stdout, stderr, code = runCLI(t, "cache", "verify", "--cache-dir="+cacheDir, "--json")
	require.Equalf(t, 0, code, "stdout=%s\nstderr=%s", stdout, stderr)
	var complete struct {
		Complete      bool     `json:"complete"`
		CachedModules []string `json:"cached_modules"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &complete))
	require.True(t, complete.Complete)
	require.ElementsMatch(t, []string{"parent@1.0.0", "child@2.0.0"}, complete.CachedModules)

	require.NoError(t, os.Remove(filepath.Join(cacheDir, "modules", "child", "2.0.0", "MODULE.bazel")))

	stdout, stderr, code = runCLI(t, "cache", "verify", "--cache-dir="+cacheDir, "--json")
	require.Equalf(t, 1, code, "stdout=%s\nstderr=%s", stdout, stderr)
	var incomplete struct {
		Complete       bool     `json:"complete"`
		MissingModules []string `json:"missing_modules"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &incomplete))
	require.False(t, incomplete.Complete)
	require.Contains(t, incomplete.MissingModules, "child@2.0.0")
}
