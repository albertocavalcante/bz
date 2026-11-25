package mod

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCmd_WithValidModuleFile(t *testing.T) {
	// Setup: create temp dir with MODULE.bazel
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")

bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "rules_python", version = "0.35.0")
bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)
`
	err := os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644)
	require.NoError(t, err)

	// Change to temp dir
	t.Chdir(tmpDir)

	// Reset flags for test isolation
	listJSON = false

	// Execute command
	var stdout bytes.Buffer
	listCmd.SetOut(&stdout)
	listCmd.SetErr(&stdout)

	err = listCmd.RunE(listCmd, []string{})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "rules_go")
	assert.Contains(t, output, "0.50.1")
	assert.Contains(t, output, "rules_python")
	assert.Contains(t, output, "gazelle")
}

func TestListCmd_WithJSONFlag(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
`
	err := os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644)
	require.NoError(t, err)

	t.Chdir(tmpDir)

	// Set JSON flag
	listJSON = true
	defer func() { listJSON = false }()

	var stdout bytes.Buffer
	listCmd.SetOut(&stdout)

	err = listCmd.RunE(listCmd, []string{})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, `"name"`)
	assert.Contains(t, output, `"rules_go"`)
	assert.Contains(t, output, `"version"`)
}

func TestListCmd_NoModuleFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Chdir(tmpDir)

	listJSON = false

	err := listCmd.RunE(listCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MODULE.bazel")
}

func TestListCmd_EmptyDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	moduleContent := `module(name = "test_module", version = "1.0.0")
`
	err := os.WriteFile(filepath.Join(tmpDir, "MODULE.bazel"), []byte(moduleContent), 0o644)
	require.NoError(t, err)

	t.Chdir(tmpDir)

	listJSON = false

	var stdout bytes.Buffer
	listCmd.SetOut(&stdout)

	err = listCmd.RunE(listCmd, []string{})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "No dependencies")
}

func TestListCmd_ExactTableOutput(t *testing.T) {
	includeWorkspace(t, map[string]string{
		"MODULE.bazel": `module(name = "demo", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.0")
include("//deps:extra.MODULE.bazel")
register_toolchains("@rules_go//go:toolchain")
register_execution_platforms("@platforms//:host")
`,
		"deps/extra.MODULE.bazel": `bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)
go = use_extension("@rules_go//go:extensions.bzl", "go", dev_dependency = True)
local_path_override(module_name = "gazelle", path = "../gz")
`,
	})
	profile := lipgloss.ColorProfile()
	t.Cleanup(func() {
		lipgloss.SetColorProfile(profile)
		listAll = false
		listJSON = false
		listCmd.SetOut(os.Stdout)
	})
	listJSON = false
	const fullStart = "demo (1.0.0)\n\nDependencies:\n  rules_go  0.50.0\n"
	const fullRest = " [deps/extra.MODULE.bazel]\n\n" +
		"Extensions:\n  go  @rules_go//go:extensions.bzl (dev) [deps/extra.MODULE.bazel]\n\n" +
		"Overrides:\n  gazelle  [local_path] → ../gz [deps/extra.MODULE.bazel]\n\n" +
		"Toolchains:\n  @rules_go//go:toolchain\n\nExecution Platforms:\n  @platforms//:host\n"

	for _, tc := range []struct {
		name    string
		profile termenv.Profile
		all     bool
		want    string
	}{
		{
			name:    "dependencies plain",
			profile: termenv.Ascii,
			want: "demo (1.0.0)\n\nNAME      VERSION  DEV  FILE\n" +
				"rules_go  0.50.0        MODULE.bazel\n" +
				"gazelle   0.38.0   yes  deps/extra.MODULE.bazel\n",
		},
		{
			name:    "dependencies ANSI",
			profile: termenv.ANSI,
			want: "demo (1.0.0)\n\nNAME      VERSION  DEV           FILE\n" +
				"rules_go  0.50.0                 MODULE.bazel\n" +
				"gazelle   0.38.0   \x1b[90myes\x1b[0m  deps/extra.MODULE.bazel\n",
		},
		{
			name:    "all plain",
			profile: termenv.Ascii,
			all:     true,
			want:    fullStart + "  gazelle   0.38.0 (dev)" + fullRest,
		},
		{
			name:    "all ANSI",
			profile: termenv.ANSI,
			all:     true,
			want:    fullStart + "  gazelle   0.38.0\x1b[90m (dev)\x1b[0m" + fullRest,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lipgloss.SetColorProfile(tc.profile)
			listAll = tc.all
			var out bytes.Buffer
			listCmd.SetOut(&out)
			require.NoError(t, listCmd.RunE(listCmd, nil))
			assert.Equal(t, tc.want, out.String())
		})
	}
}

func TestListCmd_FileColumnFollowsIncludedStatements(t *testing.T) {
	for _, tc := range []struct {
		name    string
		include string
		want    string
	}{
		{
			name:    "empty include",
			include: "# no statements here\n",
			want:    "NAME      VERSION  DEV\nrules_go  0.50.0   \n",
		},
		{
			name:    "included extension only",
			include: `go = use_extension("@rules_go//go:extensions.bzl", "go")`,
			want:    "NAME      VERSION  DEV  FILE\nrules_go  0.50.0        MODULE.bazel\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			includeWorkspace(t, map[string]string{
				"MODULE.bazel": `bazel_dep(name = "rules_go", version = "0.50.0")
include("//deps:extra.MODULE.bazel")
`,
				"deps/extra.MODULE.bazel": tc.include,
			})

			listAll = false
			listJSON = false
			var out bytes.Buffer
			listCmd.SetOut(&out)
			t.Cleanup(func() { listCmd.SetOut(os.Stdout) })

			require.NoError(t, listCmd.RunE(listCmd, nil))
			assert.Equal(t, tc.want, out.String())
		})
	}
}

func TestListCmd_EmptyFullTable(t *testing.T) {
	includeWorkspace(t, map[string]string{"MODULE.bazel": ""})
	listAll = true
	listJSON = false
	var out bytes.Buffer
	listCmd.SetOut(&out)
	t.Cleanup(func() {
		listAll = false
		listCmd.SetOut(os.Stdout)
	})

	require.NoError(t, listCmd.RunE(listCmd, nil))
	assert.Equal(t, "\nNo dependencies, extensions, or overrides found.\n", out.String())
}
