package mod

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/testutil"
)

func toCRLF(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

func writeModule(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(p, []byte(content), mode))
	require.NoError(t, os.Chmod(p, mode))
	t.Chdir(dir)
	return p
}

func TestRmCmd_MultiLineAndFormats(t *testing.T) {
	const head = "module(name = \"m\", version = \"1.0\")\n\n"
	const keep = "bazel_dep(name = \"keep\", version = \"1\")\n"
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"multi-line", head + "bazel_dep(\n    name = \"x\",\n    version = \"1\",\n)\n" + keep, head + keep},
		{"trailing comma", head + "bazel_dep(name = \"x\", version = \"1\",)\n" + keep, head + keep},
		{"attached comment", head + "bazel_dep(name = \"x\", version = \"1\")  # why\n" + keep, head + keep},
		{"dev and repo_name", head + "bazel_dep(\n  name = \"x\",\n  version = \"1\",\n  repo_name = \"xx\",\n  dev_dependency = True,\n)\n" + keep, head + keep},
		{"crlf", toCRLF(head + "bazel_dep(\n    name = \"x\",\n    version = \"1\",\n)\n" + keep), toCRLF(head + keep)},
		{"no trailing newline", head + keep + "bazel_dep(name = \"x\", version = \"1\")", head + strings.TrimSuffix(keep, "\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeModule(t, tt.in, 0o600)
			rmDryRun = false
			var out bytes.Buffer
			rmCmd.SetOut(&out)
			defer rmCmd.SetOut(os.Stdout)

			require.NoError(t, rmCmd.RunE(rmCmd, []string{"x"}))

			got, err := os.ReadFile(p)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
			assert.Contains(t, out.String(), "Removed x")
			info, err := os.Stat(p)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})
	}
}

func TestUpdateCmd_MultiLineAndFormats(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"a": {Versions: []string{"1.0.0", "1.1.0"}},
		"b": {Versions: []string{"2.0.0"}},
	})
	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()
	updateDryRun = false

	tests := []struct {
		name        string
		in          string
		want        string
		wantOut     string
		notWantOut  string
		wantChanged bool
	}{
		{
			name:        "single line",
			in:          "bazel_dep(name = \"a\", version = \"1.0.0\")\n",
			want:        "bazel_dep(name = \"a\", version = \"1.1.0\")\n",
			wantOut:     "Updated a: 1.0.0 -> 1.1.0",
			wantChanged: true,
		},
		{
			name:        "multi-line",
			in:          "bazel_dep(\n    name = \"a\",\n    version = \"1.0.0\",\n    repo_name = \"aa\",\n)\n",
			want:        "bazel_dep(\n    name = \"a\",\n    version = \"1.1.0\",\n    repo_name = \"aa\",\n)\n",
			wantOut:     "Updated a: 1.0.0 -> 1.1.0",
			wantChanged: true,
		},
		{
			name:        "dev_dependency trailing comma",
			in:          "bazel_dep(name = \"a\", version = \"1.0.0\", dev_dependency = True,)  # c\n",
			want:        "bazel_dep(name = \"a\", version = \"1.1.0\", dev_dependency = True,)  # c\n",
			wantOut:     "Updated a: 1.0.0 -> 1.1.0",
			wantChanged: true,
		},
		{
			name:        "crlf",
			in:          toCRLF("bazel_dep(\n    name = \"a\",\n    version = \"1.0.0\",\n)\n"),
			want:        toCRLF("bazel_dep(\n    name = \"a\",\n    version = \"1.1.0\",\n)\n"),
			wantOut:     "Updated a",
			wantChanged: true,
		},
		{
			name:        "no trailing newline",
			in:          "bazel_dep(name = \"a\", version = \"1.0.0\")",
			want:        "bazel_dep(name = \"a\", version = \"1.1.0\")",
			wantOut:     "Updated a",
			wantChanged: true,
		},
		{
			name:       "no version argument is skipped",
			in:         "bazel_dep(name = \"b\", dev_dependency = True)\nlocal_path_override(module_name = \"b\", path = \"../b\")\n",
			want:       "bazel_dep(name = \"b\", dev_dependency = True)\nlocal_path_override(module_name = \"b\", path = \"../b\")\n",
			wantOut:    "Skipping b: no version argument",
			notWantOut: "Updated",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeModule(t, tt.in, 0o640)
			var out bytes.Buffer
			updateCmd.SetOut(&out)
			defer updateCmd.SetOut(os.Stdout)

			require.NoError(t, updateCmd.RunE(updateCmd, nil))

			got, err := os.ReadFile(p)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
			assert.Contains(t, out.String(), tt.wantOut)
			if tt.notWantOut != "" {
				assert.NotContains(t, out.String(), tt.notWantOut)
			}
			info, err := os.Stat(p)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
		})
	}
}

func TestAddCmd_PreservesLineEndings(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"crlf", toCRLF("module(name = \"m\")\n"), toCRLF("module(name = \"m\")\nbazel_dep(name = \"a\", version = \"1\")\n")},
		{"lf", "module(name = \"m\")\n", "module(name = \"m\")\nbazel_dep(name = \"a\", version = \"1\")\n"},
		{"no trailing newline", "module(name = \"m\")", "module(name = \"m\")\nbazel_dep(name = \"a\", version = \"1\")"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeModule(t, tt.in, 0o644)
			addDev, addNoVerify, addDryRun = false, true, false
			require.NoError(t, addCmd.RunE(addCmd, []string{"a@1"}))
			got, err := os.ReadFile(p)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestAddCmd_RejectsDuplicateInOneInvocation(t *testing.T) {
	in := "module(name = \"m\")\n"
	p := writeModule(t, in, 0o644)
	addDev, addNoVerify, addDryRun = false, true, false

	err := addCmd.RunE(addCmd, []string{"a@1", "a@2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"a" specified more than once`)

	got, rerr := os.ReadFile(p)
	require.NoError(t, rerr)
	assert.Equal(t, in, string(got), "file must be untouched")
}

func TestAddCmd_WarnsOnYankedVersion(t *testing.T) {
	registryDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"a": {Versions: []string{"1.0.0", "1.1.0"}, YankedVersions: map[string]string{"1.0.0": "broken release"}},
	})
	oldRegistry := registryFlag
	registryFlag = registryDir
	defer func() { registryFlag = oldRegistry }()

	p := writeModule(t, "module(name = \"m\")\n", 0o644)
	addDev, addNoVerify, addDryRun = false, false, false

	var stderr bytes.Buffer
	addCmd.SetErr(&stderr)
	defer addCmd.SetErr(os.Stderr)

	require.NoError(t, addCmd.RunE(addCmd, []string{"a@1.0.0"}))
	assert.Contains(t, stderr.String(), "a@1.0.0 is yanked: broken release")

	got, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Contains(t, string(got), `bazel_dep(name = "a", version = "1.0.0")`)

	stderr.Reset()
	require.NoError(t, os.WriteFile(p, []byte("module(name = \"m\")\n"), 0o644))
	require.NoError(t, addCmd.RunE(addCmd, []string{"a@1.1.0"}))
	assert.NotContains(t, stderr.String(), "yanked")
}

func TestRmCmd_WarnsAboutRemainingOverride(t *testing.T) {
	in := "bazel_dep(name = \"x\", version = \"1\")\nsingle_version_override(module_name = \"x\", version = \"1\")\n"
	p := writeModule(t, in, 0o644)
	rmDryRun = false
	var stdout, stderr bytes.Buffer
	rmCmd.SetOut(&stdout)
	rmCmd.SetErr(&stderr)
	defer rmCmd.SetOut(os.Stdout)
	defer rmCmd.SetErr(os.Stderr)

	require.NoError(t, rmCmd.RunE(rmCmd, []string{"x"}))

	got, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "single_version_override(module_name = \"x\", version = \"1\")\n", string(got))
	assert.Contains(t, stderr.String(), "single_version_override for removed module x")
}

func TestRmCmd_ReadOnlyFileIsRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	in := "bazel_dep(name = \"x\", version = \"1\")\n"
	p := writeModule(t, in, 0o444)
	rmDryRun = false

	require.Error(t, rmCmd.RunE(rmCmd, []string{"x"}))

	got, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, in, string(got))
}
