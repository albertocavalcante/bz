package cache

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetStatsCmd resets the stats command state for testing.
func resetStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           statsCmd.Use,
		Short:         statsCmd.Short,
		Long:          statsCmd.Long,
		RunE:          runStats,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().BoolVar(&statsJSON, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&statsCacheDir, "cache-dir", "", "Cache directory (default: ~/.cache/bz)")
	return cmd
}

func TestStatsCmd_Help(t *testing.T) {
	statsJSON = false
	cmd := resetStatsCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "stats")
	assert.Contains(t, output, "statistics")
}

func TestStatsCmd_EmptyCache(t *testing.T) {
	cacheDir := t.TempDir()
	modulesDir := filepath.Join(cacheDir, "modules")
	require.NoError(t, os.MkdirAll(modulesDir, 0o755))

	statsJSON = false
	cmd := resetStatsCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Cache directory:")
	assert.Contains(t, output, "Total modules:")
	assert.Contains(t, output, "0")
}

func TestStatsCmd_WithModules(t *testing.T) {
	cacheDir := t.TempDir()

	// Create some mock cache entries
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 1024)
	createMockCacheEntry(t, cacheDir, "gazelle", "0.38.0", 2048)

	statsJSON = false
	cmd := resetStatsCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Cache directory:")
	assert.Contains(t, output, "Total modules:")
	assert.Contains(t, output, "2")
	assert.Contains(t, output, "Total size:")
}

func TestStatsCmd_JSONOutput(t *testing.T) {
	cacheDir := t.TempDir()

	// Create some mock cache entries
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 1024)

	statsJSON = false
	cmd := resetStatsCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--json"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Parse JSON output
	var result statsResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, cacheDir, result.CacheDir)
	assert.Equal(t, 1, result.TotalModules)
	assert.Greater(t, result.TotalSizeBytes, int64(0))
}

func TestStatsCmd_NonExistentCache(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "nonexistent")

	statsJSON = false
	cmd := resetStatsCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Cache directory:")
	assert.Contains(t, output, "Total modules:")
	assert.Contains(t, output, "0")
}

// createMockCacheEntry creates a mock cache entry for testing.
func createMockCacheEntry(t *testing.T, cacheDir, name, version string, size int) {
	t.Helper()

	moduleDir := filepath.Join(cacheDir, "modules", name)
	versionDir := filepath.Join(moduleDir, version)
	require.NoError(t, os.MkdirAll(versionDir, 0o755))

	// Create metadata.json
	metadataPath := filepath.Join(moduleDir, "metadata.json")
	metadata := []byte(`{"versions": ["` + version + `"]}`)
	require.NoError(t, os.WriteFile(metadataPath, metadata, 0o644))

	// Create MODULE.bazel with specified size
	moduleBazelPath := filepath.Join(versionDir, "MODULE.bazel")
	content := make([]byte, size)
	for i := range content {
		content[i] = 'a'
	}
	require.NoError(t, os.WriteFile(moduleBazelPath, content, 0o644))
}
