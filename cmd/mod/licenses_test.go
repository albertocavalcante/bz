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

func TestLicensesCmd_ListAllDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup registry with modules and licenses
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.50.1"}, License: "Apache-2.0"},
		"rules_python": {Versions: []string{"0.35.0"}, License: "Apache-2.0"},
		"gazelle":      {Versions: []string{"0.38.0"}, License: "Apache-2.0"},
		"protobuf":     {Versions: []string{"21.7"}, License: "BSD-3-Clause"},
	})

	// Setup MODULE.bazel with 4 direct deps
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0")
bazel_dep(name = "protobuf", version = "21.7")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	// Reset flags
	licensesJSON = false
	licensesCheck = false
	licensesSummary = false
	licensesAllow = ""
	licensesDeny = ""

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.50.1")
	assert.Contains(t, output, "Apache-2.0")
	assert.Contains(t, output, "rules_python")
	assert.Contains(t, output, "gazelle")
	assert.Contains(t, output, "protobuf")
	assert.Contains(t, output, "BSD-3-Clause")
}

func TestLicensesCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}, License: "Apache-2.0"},
		"protobuf": {Versions: []string{"21.7"}, License: "BSD-3-Clause"},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "protobuf", version = "21.7")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	licensesJSON = true
	licensesCheck = false
	licensesSummary = false
	licensesAllow = ""
	licensesDeny = ""
	defer func() { licensesJSON = false }()

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.NoError(t, err)

	// Parse JSON output
	var result struct {
		Modules []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			License string `json:"license"`
		} `json:"modules"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Len(t, result.Modules, 2)

	// Find rules_go in results
	var foundRulesGo, foundProtobuf bool
	for _, m := range result.Modules {
		if m.Name == "rules_go" {
			foundRulesGo = true
			assert.Equal(t, "0.50.1", m.Version)
			assert.Equal(t, "Apache-2.0", m.License)
		}
		if m.Name == "protobuf" {
			foundProtobuf = true
			assert.Equal(t, "21.7", m.Version)
			assert.Equal(t, "BSD-3-Clause", m.License)
		}
	}
	assert.True(t, foundRulesGo, "rules_go should be in output")
	assert.True(t, foundProtobuf, "protobuf should be in output")
}

func TestLicensesCmd_CheckWithDenyList_Pass(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}, License: "Apache-2.0"},
		"protobuf": {Versions: []string{"21.7"}, License: "BSD-3-Clause"},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "protobuf", version = "21.7")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	licensesJSON = false
	licensesCheck = true
	licensesSummary = false
	licensesAllow = ""
	licensesDeny = "GPL-3.0,AGPL-3.0"
	defer func() {
		licensesCheck = false
		licensesDeny = ""
	}()

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "All licenses are compliant")
}

func TestLicensesCmd_CheckWithDenyList_Fail(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":   {Versions: []string{"0.50.1"}, License: "Apache-2.0"},
		"gpl_module": {Versions: []string{"1.0.0"}, License: "GPL-3.0"},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "gpl_module", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	licensesJSON = false
	licensesCheck = true
	licensesSummary = false
	licensesAllow = ""
	licensesDeny = "GPL-3.0,AGPL-3.0"
	defer func() {
		licensesCheck = false
		licensesDeny = ""
	}()

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "license policy violation")
	assert.Contains(t, err.Error(), "gpl_module")
	assert.Contains(t, err.Error(), "GPL-3.0")
}

func TestLicensesCmd_CheckWithAllowList_Pass(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}, License: "Apache-2.0"},
		"protobuf": {Versions: []string{"21.7"}, License: "BSD-3-Clause"},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "protobuf", version = "21.7")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	licensesJSON = false
	licensesCheck = true
	licensesSummary = false
	licensesAllow = "Apache-2.0,BSD-3-Clause,MIT"
	licensesDeny = ""
	defer func() {
		licensesCheck = false
		licensesAllow = ""
	}()

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "All licenses are compliant")
}

func TestLicensesCmd_CheckWithAllowList_Fail(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":   {Versions: []string{"0.50.1"}, License: "Apache-2.0"},
		"gpl_module": {Versions: []string{"1.0.0"}, License: "GPL-3.0"},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "gpl_module", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	licensesJSON = false
	licensesCheck = true
	licensesSummary = false
	licensesAllow = "Apache-2.0,BSD-3-Clause,MIT"
	licensesDeny = ""
	defer func() {
		licensesCheck = false
		licensesAllow = ""
	}()

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "license policy violation")
	assert.Contains(t, err.Error(), "gpl_module")
	assert.Contains(t, err.Error(), "GPL-3.0")
}

func TestLicensesCmd_SummaryOutput(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go":     {Versions: []string{"0.50.1"}, License: "Apache-2.0"},
		"rules_python": {Versions: []string{"0.35.0"}, License: "Apache-2.0"},
		"gazelle":      {Versions: []string{"0.38.0"}, License: "Apache-2.0"},
		"protobuf":     {Versions: []string{"21.7"}, License: "BSD-3-Clause"},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0")
bazel_dep(name = "protobuf", version = "21.7")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	licensesJSON = false
	licensesCheck = false
	licensesSummary = true
	licensesAllow = ""
	licensesDeny = ""
	defer func() { licensesSummary = false }()

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Apache-2.0")
	assert.Contains(t, output, "3") // 3 Apache-2.0 licenses
	assert.Contains(t, output, "BSD-3-Clause")
	assert.Contains(t, output, "1") // 1 BSD-3-Clause license
}

func TestLicensesCmd_UnknownLicense(t *testing.T) {
	tmpDir := t.TempDir()

	// Create registry without license info (License field empty)
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}}, // No license specified
	})

	// Setup MODULE.bazel
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	licensesJSON = false
	licensesCheck = false
	licensesSummary = false
	licensesAllow = ""
	licensesDeny = ""

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "Unknown")
}

func TestLicensesCmd_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	licensesJSON = false
	licensesCheck = false
	licensesSummary = false
	licensesAllow = ""
	licensesDeny = ""

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No dependencies")
}

func TestLicensesCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	licensesJSON = false
	licensesCheck = false
	licensesSummary = false

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestLicensesCmd_CheckRequiresDenyOrAllow(t *testing.T) {
	tmpDir := t.TempDir()

	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.50.1"}, License: "Apache-2.0"},
	})

	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644))

	t.Chdir(tmpDir)

	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	licensesJSON = false
	licensesCheck = true
	licensesSummary = false
	licensesAllow = ""
	licensesDeny = ""
	defer func() { licensesCheck = false }()

	var buf bytes.Buffer
	licensesCmd.SetOut(&buf)
	defer licensesCmd.SetOut(os.Stdout)

	err := licensesCmd.RunE(licensesCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--allow or --deny")
}
