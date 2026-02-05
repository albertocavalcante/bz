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

// resetClearCmd resets the clear command state for testing.
func resetClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           clearCmd.Use,
		Short:         clearCmd.Short,
		Long:          clearCmd.Long,
		RunE:          runClear,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().BoolVar(&clearJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&clearForce, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&clearCacheDir, "cache-dir", "", "Cache directory (default: ~/.cache/bz)")
	return cmd
}

func TestClearCmd_Help(t *testing.T) {
	clearJSON = false
	clearForce = false
	cmd := resetClearCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "clear")
	assert.Contains(t, output, "Clears")
}

func TestClearCmd_EmptyCache(t *testing.T) {
	cacheDir := t.TempDir()
	modulesDir := filepath.Join(cacheDir, "modules")
	require.NoError(t, os.MkdirAll(modulesDir, 0o755))

	clearJSON = false
	clearForce = false
	cmd := resetClearCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--force"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "cleared")
}

func TestClearCmd_WithModules(t *testing.T) {
	cacheDir := t.TempDir()

	// Create some mock cache entries
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 1024)
	createMockCacheEntry(t, cacheDir, "gazelle", "0.38.0", 2048)

	// Verify entries exist
	assert.DirExists(t, filepath.Join(cacheDir, "modules", "rules_go"))
	assert.DirExists(t, filepath.Join(cacheDir, "modules", "gazelle"))

	clearJSON = false
	clearForce = false
	cmd := resetClearCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--force"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "2 modules")
	assert.Contains(t, output, "cleared")

	// Verify cache was cleared
	assert.NoDirExists(t, filepath.Join(cacheDir, "modules", "rules_go"))
	assert.NoDirExists(t, filepath.Join(cacheDir, "modules", "gazelle"))
}

func TestClearCmd_JSONOutput(t *testing.T) {
	cacheDir := t.TempDir()

	// Create some mock cache entries
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 1024)

	clearJSON = false
	clearForce = false
	cmd := resetClearCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--force", "--json"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Parse JSON output
	var result clearResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, 1, result.ModulesCleared)
	assert.Greater(t, result.BytesCleared, int64(0))
}

func TestClearCmd_NonExistentCache(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "nonexistent")

	clearJSON = false
	clearForce = false
	cmd := resetClearCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--force"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "cleared")
}

func TestClearCmd_NoForce(t *testing.T) {
	cacheDir := t.TempDir()
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 1024)

	clearJSON = false
	clearForce = false
	cmd := resetClearCmd()

	var stdout bytes.Buffer
	var stdin bytes.Buffer
	stdin.WriteString("n\n") // Simulate "no" input

	cmd.SetOut(&stdout)
	cmd.SetIn(&stdin)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir})

	err := cmd.Execute()
	// Without force and with "n" input, should abort
	assert.NoError(t, err) // Command completes without error, just doesn't clear

	// Verify cache was NOT cleared
	assert.DirExists(t, filepath.Join(cacheDir, "modules", "rules_go"))
}

func TestClearCmd_SpecificModule(t *testing.T) {
	cacheDir := t.TempDir()

	// Create some mock cache entries
	createMockCacheEntry(t, cacheDir, "rules_go", "0.50.1", 1024)
	createMockCacheEntry(t, cacheDir, "gazelle", "0.38.0", 2048)

	clearJSON = false
	clearForce = false
	cmd := resetClearCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--cache-dir=" + cacheDir, "--force", "rules_go"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify only rules_go was cleared
	assert.NoDirExists(t, filepath.Join(cacheDir, "modules", "rules_go"))
	assert.DirExists(t, filepath.Join(cacheDir, "modules", "gazelle"))
}
