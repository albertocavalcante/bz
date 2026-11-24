package module

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

func TestRemoveDeps(t *testing.T) {
	t.Parallel()

	const head = "module(name = \"m\", version = \"1.0\")\n\n# keep me\n"
	tests := []struct {
		name  string
		in    string
		names []string
		want  string
	}{
		{
			name:  "single line",
			in:    head + "bazel_dep(name = \"x\", version = \"1\")\nbazel_dep(name = \"y\", version = \"2\")\n",
			names: []string{"x"},
			want:  head + "bazel_dep(name = \"y\", version = \"2\")\n",
		},
		{
			name:  "multi line",
			in:    head + "bazel_dep(\n    name = \"x\",\n    version = \"1\",\n)\nbazel_dep(name = \"y\", version = \"2\")\n",
			names: []string{"x"},
			want:  head + "bazel_dep(name = \"y\", version = \"2\")\n",
		},
		{
			name:  "multi line no trailing comma",
			in:    head + "bazel_dep(\n    name = \"x\",\n    version = \"1\"\n)\n",
			names: []string{"x"},
			want:  head,
		},
		{
			name:  "trailing comma single line",
			in:    head + "bazel_dep(name = \"x\", version = \"1\",)\nbazel_dep(name = \"y\", version = \"2\")\n",
			names: []string{"x"},
			want:  head + "bazel_dep(name = \"y\", version = \"2\")\n",
		},
		{
			name:  "attached trailing comment",
			in:    head + "bazel_dep(name = \"x\", version = \"1\")  # pinned for CI\nbazel_dep(name = \"y\", version = \"2\")\n",
			names: []string{"x"},
			want:  head + "bazel_dep(name = \"y\", version = \"2\")\n",
		},
		{
			name:  "comment inside statement with parens and strings",
			in:    head + "bazel_dep(\n    name = \"x\",  # ) tricky (\n    version = \"1)\",\n)\nbazel_dep(name = \"y\", version = \"2\")\n",
			names: []string{"x"},
			want:  head + "bazel_dep(name = \"y\", version = \"2\")\n",
		},
		{
			name:  "dev_dependency and repo_name",
			in:    head + "bazel_dep(name = \"x\", version = \"1\", repo_name = \"xx\", dev_dependency = True)\nbazel_dep(name = \"y\", version = \"2\")\n",
			names: []string{"x"},
			want:  head + "bazel_dep(name = \"y\", version = \"2\")\n",
		},
		{
			name:  "keeps blank lines and other statements",
			in:    head + "\nbazel_dep(name = \"x\", version = \"1\")\n\nregister_toolchains(\"//:t\")\n",
			names: []string{"x"},
			want:  head + "\n\nregister_toolchains(\"//:t\")\n",
		},
		{
			name:  "several at once",
			in:    head + "bazel_dep(\n  name = \"x\",\n  version = \"1\",\n)\nbazel_dep(name = \"y\", version = \"2\")\nbazel_dep(name = \"z\", version = \"3\")\n",
			names: []string{"x", "z"},
			want:  head + "bazel_dep(name = \"y\", version = \"2\")\n",
		},
		{
			name:  "no trailing newline",
			in:    head + "bazel_dep(name = \"y\", version = \"2\")\nbazel_dep(name = \"x\", version = \"1\")",
			names: []string{"x"},
			want:  strings.TrimSuffix(head+"bazel_dep(name = \"y\", version = \"2\")\n", "\n"),
		},
		{
			name:  "crlf multi line",
			in:    crlf(head + "bazel_dep(\n    name = \"x\",\n    version = \"1\",\n)\nbazel_dep(name = \"y\", version = \"2\")\n"),
			names: []string{"x"},
			want:  crlf(head + "bazel_dep(name = \"y\", version = \"2\")\n"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, removed, err := RemoveDeps([]byte(tt.in), tt.names)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.names, removed)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestRemoveDeps_UnparsableInput(t *testing.T) {
	t.Parallel()
	_, _, err := RemoveDeps([]byte("bazel_dep(name = \"x\"\n"), []string{"x"})
	require.Error(t, err)
}

func TestRemoveDeps_UnlocatedNameIsError(t *testing.T) {
	t.Parallel()
	in := "bazel_dep(name = \"y\", version = \"2\")\n"
	out, removed, err := RemoveDeps([]byte(in), []string{"y", "nope"})
	require.ErrorIs(t, err, ErrDepNotFound)
	assert.Contains(t, err.Error(), "nope")
	assert.Nil(t, out)
	assert.Nil(t, removed)
}

func TestSetDepVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		in          string
		want        string
		wantChanged bool
		wantErr     error
	}{
		{
			name:        "single line",
			in:          "bazel_dep(name = \"a\", version = \"1.0.0\")\n",
			want:        "bazel_dep(name = \"a\", version = \"2.0.0\")\n",
			wantChanged: true,
		},
		{
			name:        "multi line, version on own line",
			in:          "bazel_dep(\n    name = \"a\",\n    version = \"1.0.0\",  # keep\n    repo_name = \"aa\",\n)\n",
			want:        "bazel_dep(\n    name = \"a\",\n    version = \"2.0.0\",  # keep\n    repo_name = \"aa\",\n)\n",
			wantChanged: true,
		},
		{
			name:        "version before name and dev_dependency",
			in:          "bazel_dep(version = \"1.0.0\", dev_dependency = True, name = \"a\")\n",
			want:        "bazel_dep(version = \"2.0.0\", dev_dependency = True, name = \"a\")\n",
			wantChanged: true,
		},
		{
			name:        "single quotes preserved",
			in:          "bazel_dep(name = 'a', version = '1.0.0')\n",
			want:        "bazel_dep(name = 'a', version = '2.0.0')\n",
			wantChanged: true,
		},
		{
			name:        "crlf",
			in:          crlf("bazel_dep(\n  name = \"a\",\n  version = \"1.0.0\",\n)\n"),
			want:        crlf("bazel_dep(\n  name = \"a\",\n  version = \"2.0.0\",\n)\n"),
			wantChanged: true,
		},
		{
			name:        "no trailing newline",
			in:          "bazel_dep(name = \"a\", version = \"1.0.0\")",
			want:        "bazel_dep(name = \"a\", version = \"2.0.0\")",
			wantChanged: true,
		},
		{
			name:        "other deps and comments untouched",
			in:          "# c\nbazel_dep(name = \"b\", version = \"1.0.0\")\nbazel_dep(name = \"a\", version = \"1.0.0\") # x\n",
			want:        "# c\nbazel_dep(name = \"b\", version = \"1.0.0\")\nbazel_dep(name = \"a\", version = \"2.0.0\") # x\n",
			wantChanged: true,
		},
		{
			name:        "already at version",
			in:          "bazel_dep(name = \"a\", version = \"2.0.0\")\n",
			want:        "bazel_dep(name = \"a\", version = \"2.0.0\")\n",
			wantChanged: false,
		},
		{
			name:    "no version argument",
			in:      "bazel_dep(name = \"a\", dev_dependency = True)\nlocal_path_override(module_name = \"a\", path = \"../a\")\n",
			wantErr: ErrNoVersionArgument,
		},
		{
			name:    "missing dep",
			in:      "bazel_dep(name = \"b\", version = \"1\")\n",
			wantErr: ErrDepNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, changed, err := SetDepVersion([]byte(tt.in), "a", "2.0.0")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantChanged, changed)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestAppendDeps(t *testing.T) {
	t.Parallel()

	specs := []DepSpec{{Name: "a", Version: "1"}, {Name: "b", Version: "2", Dev: true}}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "lf",
			in:   "module(name = \"m\")\n",
			want: "module(name = \"m\")\nbazel_dep(name = \"a\", version = \"1\")\nbazel_dep(name = \"b\", version = \"2\", dev_dependency = True)\n",
		},
		{
			name: "crlf",
			in:   crlf("module(name = \"m\")\n"),
			want: crlf("module(name = \"m\")\nbazel_dep(name = \"a\", version = \"1\")\nbazel_dep(name = \"b\", version = \"2\", dev_dependency = True)\n"),
		},
		{
			name: "no trailing newline",
			in:   "module(name = \"m\")",
			want: "module(name = \"m\")\nbazel_dep(name = \"a\", version = \"1\")\nbazel_dep(name = \"b\", version = \"2\", dev_dependency = True)",
		},
		{
			name: "crlf no trailing newline",
			in:   strings.TrimSuffix(crlf("module(name = \"m\")\nregister_toolchains(\"//:t\")\n"), "\r\n"),
			want: crlf("module(name = \"m\")\nregister_toolchains(\"//:t\")\nbazel_dep(name = \"a\", version = \"1\")\nbazel_dep(name = \"b\", version = \"2\", dev_dependency = True)"),
		},
		{
			name: "empty file",
			in:   "",
			want: "bazel_dep(name = \"a\", version = \"1\")\nbazel_dep(name = \"b\", version = \"2\", dev_dependency = True)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := AppendDeps([]byte(tt.in), specs)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestDetectEOL(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "\n", DetectEOL(nil))
	assert.Equal(t, "\n", DetectEOL([]byte("a\nb\n")))
	assert.Equal(t, "\r\n", DetectEOL([]byte("a\r\nb\r\n")))
	assert.Equal(t, "\n", DetectEOL([]byte("a\r\nb\nc\n")))
}

func TestWriteFileAtomic(t *testing.T) {
	t.Parallel()

	t.Run("replaces content and preserves mode", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		p := filepath.Join(dir, "MODULE.bazel")
		require.NoError(t, os.WriteFile(p, []byte("old"), 0o600))
		require.NoError(t, os.Chmod(p, 0o600))

		require.NoError(t, WriteFileAtomic(p, []byte("new")))

		got, err := os.ReadFile(p)
		require.NoError(t, err)
		assert.Equal(t, "new", string(got))
		info, err := os.Stat(p)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "no temp files left behind")
	})

	t.Run("failed write leaves original intact", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("directory permissions are not enforced")
		}
		dir := t.TempDir()
		p := filepath.Join(dir, "MODULE.bazel")
		require.NoError(t, os.WriteFile(p, []byte("old"), 0o644))
		require.NoError(t, os.Chmod(dir, 0o500))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

		require.Error(t, WriteFileAtomic(p, []byte("new")))

		got, err := os.ReadFile(p)
		require.NoError(t, err)
		assert.Equal(t, "old", string(got))
	})

	t.Run("follows symlinks", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		realPath := filepath.Join(dir, "real.bazel")
		link := filepath.Join(dir, "MODULE.bazel")
		require.NoError(t, os.WriteFile(realPath, []byte("old"), 0o644))
		if err := os.Symlink("real.bazel", link); err != nil {
			t.Skip("symlinks unavailable")
		}
		require.NoError(t, WriteFileAtomic(link, []byte("new")))
		fi, err := os.Lstat(link)
		require.NoError(t, err)
		assert.NotZero(t, fi.Mode()&os.ModeSymlink)
		got, _ := os.ReadFile(realPath)
		assert.Equal(t, "new", string(got))
	})
}

func TestWriteFileAtomic_RefusesReadOnlyFile(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(p, []byte("old"), 0o444))
	require.NoError(t, os.Chmod(p, 0o444))

	require.Error(t, WriteFileAtomic(p, []byte("new")))

	got, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "old", string(got))
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o444), info.Mode().Perm())
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temp file left")
}

func TestWriteFileIfUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "MODULE.bazel")
	require.NoError(t, os.WriteFile(p, []byte("v1"), 0o644))

	require.NoError(t, WriteFileIfUnchanged(p, []byte("v1"), []byte("v2")))
	got, _ := os.ReadFile(p)
	assert.Equal(t, "v2", string(got))

	// Simulate an editor save between our read and our write.
	require.NoError(t, os.WriteFile(p, []byte("edited"), 0o644))
	err := WriteFileIfUnchanged(p, []byte("v2"), []byte("v3"))
	require.ErrorIs(t, err, ErrFileChanged)
	got, _ = os.ReadFile(p)
	assert.Equal(t, "edited", string(got))
}

func TestOverridesFor(t *testing.T) {
	t.Parallel()
	f, err := LoadContent("MODULE.bazel", []byte("bazel_dep(name = \"x\", version = \"1\")\nsingle_version_override(module_name = \"x\", version = \"1\")\nlocal_path_override(module_name = \"y\", path = \"../y\")\n"))
	require.NoError(t, err)
	got := f.OverridesFor("x")
	require.Len(t, got, 1)
	assert.Equal(t, "single_version", got[0].Type)
	assert.Empty(t, f.OverridesFor("z"))
}

func TestErrorsAreDistinct(t *testing.T) {
	t.Parallel()
	assert.False(t, errors.Is(ErrDepNotFound, ErrNoVersionArgument))
}
