package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/cli"
)

func setupSBOMTest(t *testing.T, moduleContent string) string {
	t.Helper()

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(moduleContent),
		0o644,
	))

	t.Chdir(tmpDir)

	return tmpDir
}

// setupSBOMTestRegistry creates a file-based registry with module dependencies.
func setupSBOMTestRegistry(t *testing.T, tmpDir string, modules map[string]sbomModuleInfo) string {
	t.Helper()

	registryDir := filepath.Join(tmpDir, "registry")
	modulesDir := filepath.Join(registryDir, "modules")
	require.NoError(t, os.MkdirAll(modulesDir, 0o755))

	for name, info := range modules {
		moduleDir := filepath.Join(modulesDir, name)
		require.NoError(t, os.MkdirAll(moduleDir, 0o755))

		// Create metadata.json
		metadata := map[string]any{
			"versions": info.versions,
		}
		metadataBytes, err := json.Marshal(metadata)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(
			filepath.Join(moduleDir, "metadata.json"),
			metadataBytes,
			0o644,
		))

		// Create version directories with MODULE.bazel including dependencies
		for _, ver := range info.versions {
			verDir := filepath.Join(moduleDir, ver)
			require.NoError(t, os.MkdirAll(verDir, 0o755))

			// Build MODULE.bazel content with dependencies
			var content strings.Builder
			content.WriteString("module(name = \"" + name + "\", version = \"" + ver + "\")\n")
			if deps, ok := info.deps[ver]; ok {
				for _, dep := range deps {
					content.WriteString("bazel_dep(name = \"" + dep.name + "\", version = \"" + dep.version + "\")\n")
				}
			}
			require.NoError(t, os.WriteFile(
				filepath.Join(verDir, "MODULE.bazel"),
				[]byte(content.String()),
				0o644,
			))

			// Create source.json with download location
			sourceJSON := map[string]any{
				"url": "https://bcr.bazel.build/modules/" + name + "/" + ver + "/source.tar.gz",
			}
			sourceBytes, err := json.Marshal(sourceJSON)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(
				filepath.Join(verDir, "source.json"),
				sourceBytes,
				0o644,
			))
		}
	}

	return registryDir
}

type sbomModuleInfo struct {
	versions []string
	deps     map[string][]sbomDepInfo // version -> dependencies
}

type sbomDepInfo struct {
	name    string
	version string
}

func resetSBOMFlags() {
	sbomFormat = "spdx"
	sbomOutput = ""
	sbomIncludeTransitive = true
	cli.Global.Registry = ""
}

func TestSBOMCmd_SPDXFormat(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
`)

	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"rules_go": {
			versions: []string{"0.50.1"},
			deps:     map[string][]sbomDepInfo{},
		},
		"rules_python": {
			versions: []string{"0.35.0"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "spdx"

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var spdx SPDXDocument
	require.NoError(t, json.Unmarshal(buf.Bytes(), &spdx))

	assert.Equal(t, "SPDX-2.3", spdx.SPDXVersion)
	assert.Equal(t, "CC0-1.0", spdx.DataLicense)
	assert.Equal(t, "SPDXRef-DOCUMENT", spdx.SPDXID)
	assert.Equal(t, "my_module", spdx.Name)

	// Should have packages for both dependencies
	assert.GreaterOrEqual(t, len(spdx.Packages), 2)

	// Find rules_go package
	var foundRulesGo, foundRulesPython bool
	for _, pkg := range spdx.Packages {
		if pkg.Name == "rules_go" {
			foundRulesGo = true
			assert.Equal(t, "0.50.1", pkg.VersionInfo)
			assert.Contains(t, pkg.SPDXID, "SPDXRef-Package-rules")
		}
		if pkg.Name == "rules_python" {
			foundRulesPython = true
			assert.Equal(t, "0.35.0", pkg.VersionInfo)
		}
	}
	assert.True(t, foundRulesGo, "should include rules_go package")
	assert.True(t, foundRulesPython, "should include rules_python package")
}

func TestSBOMCmd_CycloneDXFormat(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
`)

	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"rules_go": {
			versions: []string{"0.50.1"},
			deps:     map[string][]sbomDepInfo{},
		},
		"rules_python": {
			versions: []string{"0.35.0"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "cyclonedx"

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var cdx CycloneDXBOM
	require.NoError(t, json.Unmarshal(buf.Bytes(), &cdx))

	assert.Equal(t, "CycloneDX", cdx.BOMFormat)
	assert.Equal(t, "1.4", cdx.SpecVersion)
	assert.Equal(t, 1, cdx.Version)

	// Should have components for both dependencies
	assert.GreaterOrEqual(t, len(cdx.Components), 2)

	// Find rules_go component
	var foundRulesGo, foundRulesPython bool
	for _, comp := range cdx.Components {
		if comp.Name == "rules_go" {
			foundRulesGo = true
			assert.Equal(t, "0.50.1", comp.Version)
			assert.Equal(t, "library", comp.Type)
			assert.Equal(t, "pkg:bazel/rules_go@0.50.1", comp.PURL)
		}
		if comp.Name == "rules_python" {
			foundRulesPython = true
			assert.Equal(t, "0.35.0", comp.Version)
			assert.Equal(t, "pkg:bazel/rules_python@0.35.0", comp.PURL)
		}
	}
	assert.True(t, foundRulesGo, "should include rules_go component")
	assert.True(t, foundRulesPython, "should include rules_python component")
}

func TestSBOMCmd_IncludeTransitiveDeps(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)

	// rules_go depends on platforms
	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"rules_go": {
			versions: []string{"0.50.1"},
			deps: map[string][]sbomDepInfo{
				"0.50.1": {{name: "platforms", version: "0.0.10"}},
			},
		},
		"platforms": {
			versions: []string{"0.0.10"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "spdx"
	sbomIncludeTransitive = true

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var spdx SPDXDocument
	require.NoError(t, json.Unmarshal(buf.Bytes(), &spdx))

	// Should include both direct and transitive deps
	var foundPlatforms bool
	for _, pkg := range spdx.Packages {
		if pkg.Name == "platforms" {
			foundPlatforms = true
			assert.Equal(t, "0.0.10", pkg.VersionInfo)
		}
	}
	assert.True(t, foundPlatforms, "should include transitive dependency 'platforms'")
}

func TestSBOMCmd_ExcludeTransitiveDeps(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)

	// rules_go depends on platforms
	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"rules_go": {
			versions: []string{"0.50.1"},
			deps: map[string][]sbomDepInfo{
				"0.50.1": {{name: "platforms", version: "0.0.10"}},
			},
		},
		"platforms": {
			versions: []string{"0.0.10"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "spdx"
	sbomIncludeTransitive = false

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var spdx SPDXDocument
	require.NoError(t, json.Unmarshal(buf.Bytes(), &spdx))

	// Should only include direct deps, not transitive
	var foundRulesGo, foundPlatforms bool
	for _, pkg := range spdx.Packages {
		if pkg.Name == "rules_go" {
			foundRulesGo = true
		}
		if pkg.Name == "platforms" {
			foundPlatforms = true
		}
	}
	assert.True(t, foundRulesGo, "should include direct dependency 'rules_go'")
	assert.False(t, foundPlatforms, "should NOT include transitive dependency 'platforms' when --include-transitive=false")
}

func TestSBOMCmd_OutputToFile(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)

	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"rules_go": {
			versions: []string{"0.50.1"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "spdx"
	outputFile := filepath.Join(tmpDir, "sbom.json")
	sbomOutput = outputFile

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	// Verify file was created
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	var spdx SPDXDocument
	require.NoError(t, json.Unmarshal(content, &spdx))
	assert.Equal(t, "SPDX-2.3", spdx.SPDXVersion)
}

func TestSBOMCmd_InvalidFormat(t *testing.T) {
	setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)

	resetSBOMFlags()
	sbomFormat = "invalid"

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "format")
}

func TestSBOMCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	resetSBOMFlags()

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestSBOMCmd_NoDeps(t *testing.T) {
	setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")
`)

	resetSBOMFlags()
	sbomFormat = "spdx"

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var spdx SPDXDocument
	require.NoError(t, json.Unmarshal(buf.Bytes(), &spdx))

	// Should still have valid SPDX document with just the root module
	assert.Equal(t, "SPDX-2.3", spdx.SPDXVersion)
	assert.Equal(t, "my_module", spdx.Name)
	// Packages array may be empty or have just the root
}

func TestSBOMCmd_SPDXHasDownloadLocation(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)

	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"rules_go": {
			versions: []string{"0.50.1"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "spdx"

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var spdx SPDXDocument
	require.NoError(t, json.Unmarshal(buf.Bytes(), &spdx))

	// Find rules_go package and check download location
	for _, pkg := range spdx.Packages {
		if pkg.Name == "rules_go" {
			assert.NotEmpty(t, pkg.DownloadLocation)
			assert.Contains(t, pkg.DownloadLocation, "rules_go")
		}
	}
}

func TestSBOMCmd_CycloneDXHasPURL(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)

	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"rules_go": {
			versions: []string{"0.50.1"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "cyclonedx"

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var cdx CycloneDXBOM
	require.NoError(t, json.Unmarshal(buf.Bytes(), &cdx))

	// Find rules_go component and check PURL
	for _, comp := range cdx.Components {
		if comp.Name == "rules_go" {
			assert.Equal(t, "pkg:bazel/rules_go@0.50.1", comp.PURL)
		}
	}
}

func TestSBOMCmd_DeepTransitiveDeps(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "mod_a", version = "1.0.0")
`)

	// mod_a -> mod_b -> mod_c (deep transitive)
	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"mod_a": {
			versions: []string{"1.0.0"},
			deps: map[string][]sbomDepInfo{
				"1.0.0": {{name: "mod_b", version: "1.0.0"}},
			},
		},
		"mod_b": {
			versions: []string{"1.0.0"},
			deps: map[string][]sbomDepInfo{
				"1.0.0": {{name: "mod_c", version: "1.0.0"}},
			},
		},
		"mod_c": {
			versions: []string{"1.0.0"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "spdx"
	sbomIncludeTransitive = true

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var spdx SPDXDocument
	require.NoError(t, json.Unmarshal(buf.Bytes(), &spdx))

	// Should include all levels of transitive deps
	var foundA, foundB, foundC bool
	for _, pkg := range spdx.Packages {
		switch pkg.Name {
		case "mod_a":
			foundA = true
		case "mod_b":
			foundB = true
		case "mod_c":
			foundC = true
		}
	}
	assert.True(t, foundA, "should include mod_a")
	assert.True(t, foundB, "should include mod_b (transitive)")
	assert.True(t, foundC, "should include mod_c (deep transitive)")
}

func TestSBOMCmd_CycloneDXHasMetadata(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)

	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"rules_go": {
			versions: []string{"0.50.1"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "cyclonedx"

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var cdx CycloneDXBOM
	require.NoError(t, json.Unmarshal(buf.Bytes(), &cdx))

	// Should have metadata with tool info
	require.NotNil(t, cdx.Metadata)
	require.NotEmpty(t, cdx.Metadata.Tools)
	assert.Equal(t, "bz", cdx.Metadata.Tools[0].Name)

	// Should have root component in metadata
	require.NotNil(t, cdx.Metadata.Component)
	assert.Equal(t, "my_module", cdx.Metadata.Component.Name)
	assert.Equal(t, "1.0.0", cdx.Metadata.Component.Version)
}

func TestSBOMCmd_SPDXHasCreationInfo(t *testing.T) {
	tmpDir := setupSBOMTest(t, `module(name = "my_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
`)

	registryDir := setupSBOMTestRegistry(t, tmpDir, map[string]sbomModuleInfo{
		"rules_go": {
			versions: []string{"0.50.1"},
			deps:     map[string][]sbomDepInfo{},
		},
	})

	resetSBOMFlags()
	sbomFormat = "spdx"

	oldRegistry := cli.Global.Registry
	cli.Global.Registry = registryDir
	t.Cleanup(func() { cli.Global.Registry = oldRegistry })

	var buf bytes.Buffer
	sbomCmd.SetOut(&buf)
	t.Cleanup(func() { sbomCmd.SetOut(os.Stdout) })

	err := sbomCmd.RunE(sbomCmd, []string{})
	require.NoError(t, err)

	var spdx SPDXDocument
	require.NoError(t, json.Unmarshal(buf.Bytes(), &spdx))

	// Should have creation info
	assert.NotEmpty(t, spdx.CreationInfo.Created)
	assert.NotEmpty(t, spdx.CreationInfo.Creators)

	// Should have tool creator
	var foundToolCreator bool
	for _, creator := range spdx.CreationInfo.Creators {
		if creator == "Tool: bz" {
			foundToolCreator = true
		}
	}
	assert.True(t, foundToolCreator, "should have 'Tool: bz' in creators")
}
