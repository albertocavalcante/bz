package mod

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestGraphCmd_ASCIIOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with dependency hierarchy:
	// rules_go@0.50.1 -> platforms@0.0.10
	// rules_python@0.35.0 -> rules_cc@0.0.9, platforms@0.0.10
	// gazelle@0.38.0 -> rules_go@0.50.1
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps:     map[string][]string{"0.50.1": {"platforms@0.0.10"}},
		},
		"rules_python": {
			Versions: []string{"0.35.0"},
			Deps:     map[string][]string{"0.35.0": {"rules_cc@0.0.9", "platforms@0.0.10"}},
		},
		"gazelle": {
			Versions: []string{"0.38.0"},
			Deps:     map[string][]string{"0.38.0": {"rules_go@0.50.1"}},
		},
		"platforms": {Versions: []string{"0.0.10"}},
		"rules_cc":  {Versions: []string{"0.0.9"}},
	})

	// Setup MODULE.bazel
	moduleContent := `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	// Set registry flag
	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	// Reset flags
	graphFormat = "ascii"
	graphJSON = false
	graphDepth = 0

	var buf bytes.Buffer
	graphCmd.SetOut(&buf)
	defer graphCmd.SetOut(os.Stdout)

	err := graphCmd.RunE(graphCmd, []string{})
	require.NoError(t, err)

	output := buf.String()

	// Check tree structure elements
	assert.Contains(t, output, "my_module")
	assert.Contains(t, output, "rules_go@0.50.1")
	assert.Contains(t, output, "platforms@0.0.10")
	assert.Contains(t, output, "rules_python@0.35.0")
	assert.Contains(t, output, "rules_cc@0.0.9")
	assert.Contains(t, output, "gazelle@0.38.0")

	// Check tree formatting characters are present
	assert.Contains(t, output, "\u251c\u2500\u2500")
	assert.Contains(t, output, "\u2514\u2500\u2500")
}

func TestGraphCmd_DOTOutput(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps:     map[string][]string{"0.50.1": {"platforms@0.0.10"}},
		},
		"platforms": {Versions: []string{"0.0.10"}},
	})

	moduleContent := `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	graphFormat = "dot"
	graphJSON = false
	graphDepth = 0
	defer func() { graphFormat = "ascii" }()

	var buf bytes.Buffer
	graphCmd.SetOut(&buf)
	defer graphCmd.SetOut(os.Stdout)

	err := graphCmd.RunE(graphCmd, []string{})
	require.NoError(t, err)

	output := buf.String()

	// Check DOT format
	assert.Contains(t, output, "digraph")
	assert.Contains(t, output, "->")
	assert.Contains(t, output, "my_module")
	assert.Contains(t, output, "rules_go@0.50.1")
	assert.Contains(t, output, "platforms@0.0.10")
}

func TestGraphCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps:     map[string][]string{"0.50.1": {"platforms@0.0.10"}},
		},
		"platforms": {Versions: []string{"0.0.10"}},
	})

	moduleContent := `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	graphFormat = "json"
	graphJSON = false
	graphDepth = 0
	defer func() { graphFormat = "ascii" }()

	var buf bytes.Buffer
	graphCmd.SetOut(&buf)
	defer graphCmd.SetOut(os.Stdout)

	err := graphCmd.RunE(graphCmd, []string{})
	require.NoError(t, err)

	// Verify JSON parses correctly
	var result GraphOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "my_module", result.Root.Name)
	assert.NotEmpty(t, result.Root.Dependencies)

	// Find rules_go in dependencies
	var foundRulesGo bool
	for _, dep := range result.Root.Dependencies {
		if dep.Name == "rules_go" && dep.Version == "0.50.1" {
			foundRulesGo = true
			// Check it has platforms as dependency
			assert.NotEmpty(t, dep.Dependencies)
		}
	}
	assert.True(t, foundRulesGo, "rules_go should be in dependencies")
}

func TestGraphCmd_JSONFlag(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}},
	})

	moduleContent := `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	// Test --json flag shortcut
	graphFormat = "ascii"
	graphJSON = true
	graphDepth = 0
	defer func() { graphJSON = false }()

	var buf bytes.Buffer
	graphCmd.SetOut(&buf)
	defer graphCmd.SetOut(os.Stdout)

	err := graphCmd.RunE(graphCmd, []string{})
	require.NoError(t, err)

	// Should output JSON even though format is ascii
	var result GraphOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "my_module", result.Root.Name)
}

func TestGraphCmd_DepthLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup a deep dependency chain: a -> b -> c -> d
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"mod_b": {
			Versions: []string{"1.0.0"},
			Deps:     map[string][]string{"1.0.0": {"mod_c@1.0.0"}},
		},
		"mod_c": {
			Versions: []string{"1.0.0"},
			Deps:     map[string][]string{"1.0.0": {"mod_d@1.0.0"}},
		},
		"mod_d": {Versions: []string{"1.0.0"}},
	})

	moduleContent := `module(name = "mod_a", version = "1.0.0")

bazel_dep(name = "mod_b", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	// Test with depth=1 (only direct deps)
	graphFormat = "ascii"
	graphJSON = false
	graphDepth = 1
	defer func() { graphDepth = 0 }()

	var buf bytes.Buffer
	graphCmd.SetOut(&buf)
	defer graphCmd.SetOut(os.Stdout)

	err := graphCmd.RunE(graphCmd, []string{})
	require.NoError(t, err)

	output := buf.String()

	// Should have mod_a and mod_b
	assert.Contains(t, output, "mod_a")
	assert.Contains(t, output, "mod_b@1.0.0")

	// Should NOT have mod_c or mod_d (beyond depth 1)
	assert.NotContains(t, output, "mod_c")
	assert.NotContains(t, output, "mod_d")
}

func TestGraphCmd_CycleDetection(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup a cycle: a -> b -> c -> a (cycle back to a)
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"mod_b": {
			Versions: []string{"1.0.0"},
			Deps:     map[string][]string{"1.0.0": {"mod_c@1.0.0"}},
		},
		"mod_c": {
			Versions: []string{"1.0.0"},
			Deps:     map[string][]string{"1.0.0": {"mod_a@1.0.0"}}, // Cycle back to mod_a
		},
		"mod_a": {Versions: []string{"1.0.0"}}, // In registry, mod_a has no deps (but our local one does)
	})

	moduleContent := `module(name = "mod_a", version = "1.0.0")

bazel_dep(name = "mod_b", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	graphFormat = "ascii"
	graphJSON = false
	graphDepth = 0

	var buf bytes.Buffer
	graphCmd.SetOut(&buf)
	defer graphCmd.SetOut(os.Stdout)

	// Should not panic or hang - should detect cycle
	err := graphCmd.RunE(graphCmd, []string{})
	require.NoError(t, err)

	output := buf.String()

	// Should contain cycle indicator
	assert.Contains(t, output, "mod_a")
	assert.Contains(t, output, "mod_b")
	assert.Contains(t, output, "mod_c")
	// Should indicate cycle (either with "(cycle)" marker or by simply not recursing)
	assert.Contains(t, output, "cycle")
}

func TestGraphCmd_MermaidOutput(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions: []string{"0.50.1"},
			Deps:     map[string][]string{"0.50.1": {"platforms@0.0.10"}},
		},
		"platforms": {Versions: []string{"0.0.10"}},
	})

	moduleContent := `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	graphFormat = "mermaid"
	graphJSON = false
	graphDepth = 0
	defer func() { graphFormat = "ascii" }()

	var buf bytes.Buffer
	graphCmd.SetOut(&buf)
	defer graphCmd.SetOut(os.Stdout)

	err := graphCmd.RunE(graphCmd, []string{})
	require.NoError(t, err)

	output := buf.String()

	// Check Mermaid format
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "-->")
	assert.Contains(t, output, "my_module")
}

func TestGraphCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	graphFormat = "ascii"
	graphJSON = false
	graphDepth = 0

	err := graphCmd.RunE(graphCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestGraphCmd_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	moduleContent := `module(name = "my_module", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	graphFormat = "ascii"
	graphJSON = false
	graphDepth = 0

	var buf bytes.Buffer
	graphCmd.SetOut(&buf)
	defer graphCmd.SetOut(os.Stdout)

	err := graphCmd.RunE(graphCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "my_module")
	// With no deps, should just show the root
}

func TestGraphCmd_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()

	moduleContent := `module(name = "my_module", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	graphFormat = "invalid"
	graphJSON = false
	graphDepth = 0
	defer func() { graphFormat = "ascii" }()

	err := graphCmd.RunE(graphCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "format")
}
