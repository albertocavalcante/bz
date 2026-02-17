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

	"github.com/albertocavalcante/bz/internal/testutil"
)

// resetDownloadCmd resets the download command state for testing.
func resetDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           downloadCmd.Use,
		Short:         downloadCmd.Short,
		Long:          downloadCmd.Long,
		RunE:          runDownload,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().BoolVar(&downloadAll, "all", false, "Download ALL modules from registry (warning: large)")
	cmd.Flags().BoolVar(&downloadJSON, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&downloadRegistry, "registry", "", "Registry URL to download from")
	cmd.Flags().StringVar(&downloadCacheDir, "cache-dir", "", "Cache directory (default: ~/.cache/bz)")
	return cmd
}

func TestDownloadCmd_Help(t *testing.T) {
	// Reset flags
	downloadAll = false
	downloadJSON = false

	cmd := resetDownloadCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "download")
	assert.Contains(t, output, "Downloads modules")
	assert.Contains(t, output, "MODULE.bazel")
}

func TestDownloadCmd_SpecificModules(t *testing.T) {
	// Set up a test registry with some modules
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.49.0", "0.50.1"},
		},
		"gazelle": {
			Versions: []string{"0.38.0"},
		},
	})

	// Set up a temporary cache directory
	cacheDir := t.TempDir()

	// Reset flags for test
	downloadAll = false
	downloadJSON = false

	cmd := resetDownloadCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		"--registry=" + registryDir,
		"--cache-dir=" + cacheDir,
		"rules_go@0.50.1",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go@0.50.1")
	assert.Contains(t, output, "Downloaded")

	// Verify cache structure was created
	cachedMetadata := filepath.Join(cacheDir, "modules", "rules_go", "metadata.json")
	assert.FileExists(t, cachedMetadata)

	cachedModuleBazel := filepath.Join(cacheDir, "modules", "rules_go", "0.50.1", "MODULE.bazel")
	assert.FileExists(t, cachedModuleBazel)
}

func TestDownloadCmd_ModuleFromMODULEBazel(t *testing.T) {
	// Set up a test registry with modules
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
		},
		"gazelle": {
			Versions: []string{"0.38.0"},
			Deps: map[string][]string{
				"0.38.0": {"rules_go@0.50.1"},
			},
		},
	})

	// Create a temporary project directory with MODULE.bazel
	projectDir := t.TempDir()
	moduleBazelPath := filepath.Join(projectDir, "MODULE.bazel")
	moduleBazelContent := `module(name = "test_project", version = "1.0.0")
bazel_dep(name = "gazelle", version = "0.38.0")
`
	err := os.WriteFile(moduleBazelPath, []byte(moduleBazelContent), 0o644)
	require.NoError(t, err)

	// Change to project directory for test
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Chdir(oldWd)) }()
	require.NoError(t, os.Chdir(projectDir))

	// Set up a temporary cache directory
	cacheDir := t.TempDir()

	// Reset flags for test
	downloadAll = false
	downloadJSON = false

	cmd := resetDownloadCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		"--registry=" + registryDir,
		"--cache-dir=" + cacheDir,
	})

	err = cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "gazelle@0.38.0")
	// Should also download transitive dep
	assert.Contains(t, output, "rules_go@0.50.1")
}

func TestDownloadCmd_AllModules(t *testing.T) {
	// Set up a test registry
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
		},
		"gazelle": {
			Versions: []string{"0.38.0"},
		},
	})

	// Set up a temporary cache directory
	cacheDir := t.TempDir()

	// Reset flags for test
	downloadAll = false
	downloadJSON = false

	cmd := resetDownloadCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		"--registry=" + registryDir,
		"--cache-dir=" + cacheDir,
		"--all",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "gazelle")

	// Verify both modules were cached
	assert.FileExists(t, filepath.Join(cacheDir, "modules", "rules_go", "metadata.json"))
	assert.FileExists(t, filepath.Join(cacheDir, "modules", "gazelle", "metadata.json"))
}

func TestDownloadCmd_JSONOutput(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
		},
	})

	cacheDir := t.TempDir()

	// Reset flags for test
	downloadAll = false
	downloadJSON = false

	cmd := resetDownloadCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		"--registry=" + registryDir,
		"--cache-dir=" + cacheDir,
		"--json",
		"rules_go@0.50.1",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	// Parse JSON output
	var result downloadResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Downloaded)
	assert.Contains(t, result.Modules, "rules_go@0.50.1")
}

func TestDownloadCmd_NoRegistry(t *testing.T) {
	cacheDir := t.TempDir()

	// Reset flags for test
	downloadAll = false
	downloadJSON = false

	cmd := resetDownloadCmd()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"--registry=/nonexistent/registry",
		"--cache-dir=" + cacheDir,
		"rules_go@0.50.1",
	})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestParseModuleArg(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{input: "rules_go@0.50.1", wantName: "rules_go", wantVersion: "0.50.1"},
		{input: "rules_go", wantName: "rules_go", wantVersion: ""},
		{input: "@scope/pkg", wantName: "@scope/pkg", wantVersion: ""},
		{input: "@scope/pkg@1.2.3", wantName: "@scope/pkg", wantVersion: "1.2.3"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			gotName, gotVersion := parseModuleArg(tt.input)
			assert.Equal(t, tt.wantName, gotName)
			assert.Equal(t, tt.wantVersion, gotVersion)
		})
	}
}
