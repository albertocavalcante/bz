package mod

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// setupTestRegistry creates a file-based registry with modules and their versions.
// Each module entry is: name -> []versions.
func setupTestRegistry(t *testing.T, tmpDir string, modules map[string][]string) string {
	t.Helper()

	registryDir := filepath.Join(tmpDir, "registry")
	modulesDir := filepath.Join(registryDir, "modules")
	require.NoError(t, os.MkdirAll(modulesDir, 0o755))

	for name, versions := range modules {
		moduleDir := filepath.Join(modulesDir, name)
		require.NoError(t, os.MkdirAll(moduleDir, 0o755))

		// Sort versions for consistent metadata
		sortedVersions := make([]string, len(versions))
		copy(sortedVersions, versions)
		sort.Strings(sortedVersions)

		// Create metadata.json
		metadata := map[string]any{
			"versions": sortedVersions,
		}
		metadataBytes, err := json.Marshal(metadata)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "metadata.json"), metadataBytes, 0o644))

		// Create version directories with MODULE.bazel
		for _, ver := range versions {
			verDir := filepath.Join(moduleDir, ver)
			require.NoError(t, os.MkdirAll(verDir, 0o755))

			// Create MODULE.bazel
			var moduleBazel strings.Builder
			moduleBazel.WriteString(`module(name = "`)
			moduleBazel.WriteString(name)
			moduleBazel.WriteString(`", version = "`)
			moduleBazel.WriteString(ver)
			moduleBazel.WriteString("\")\n")
			require.NoError(t, os.WriteFile(filepath.Join(verDir, "MODULE.bazel"), []byte(moduleBazel.String()), 0o644))
		}
	}

	return registryDir
}
