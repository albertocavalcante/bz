package workspace_test

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/workspace"
)

func ExamplePreviewVersion() {
	tree := fstest.MapFS{
		"MODULE.bazel": {Data: []byte("bazel_dep(name = \"rules_go\", version = \"0.50.0\")\n")},
	}
	edit, err := workspace.PreviewVersion(context.Background(), tree, "rules_go", "0.51.0")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%s:%d:%d\n%s", edit.Path, edit.Line, edit.Column, edit.After)
	// Output:
	// MODULE.bazel:1:1
	// bazel_dep(name = "rules_go", version = "0.51.0")
}

func TestPreviewVersionCapturesIncludedFile(t *testing.T) {
	t.Parallel()

	const path = "dep/rules.MODULE.bazel"
	const before = "# comment with a closing paren )\r\nbazel_dep(\r\n    name = 'a',\r\n    version = '1.0.0',  # keep this\r\n)\r\n# end"
	tree := &trackingFS{
		MapFS: fstest.MapFS{
			"MODULE.bazel":        {Data: []byte("include(\"//dep:rules.MODULE.bazel\")\ninclude(\"//:nested.MODULE.bazel\")\n")},
			"nested.MODULE.bazel": {Data: []byte("include(\"//dep:rules.MODULE.bazel\")\n")},
			path:                  {Data: []byte(before)},
			"unused.MODULE.bazel": {Data: []byte("not opened")},
		},
		opens: make(map[string]int),
	}
	edit, err := workspace.PreviewVersion(t.Context(), tree, "a", "2.0.0")
	require.NoError(t, err)
	require.Equal(t, workspace.Edit{
		Path: path, Line: 2, Column: 1, CurrentVersion: "1.0.0", Changed: true,
		Before: []byte(before), After: []byte(strings.Replace(before, "'1.0.0'", "'2.0.0'", 1)),
	}, edit)
	require.Equal(t, map[string]int{"MODULE.bazel": 1, "nested.MODULE.bazel": 1, path: 1}, tree.opens)

	edit.Before[0] = '!'
	require.Equal(t, byte('#'), edit.After[0])
	edit.After[0] = '?'
	require.Equal(t, before, string(tree.MapFS[path].Data))
}

type trackingFS struct {
	fstest.MapFS
	opens      map[string]int
	beforeOpen func()
}

func (f *trackingFS) Open(name string) (fs.File, error) {
	f.opens[name]++
	if f.beforeOpen != nil {
		f.beforeOpen()
	}
	return f.MapFS.Open(name)
}

func TestPreviewVersionNoOpPreservesLiteralSpelling(t *testing.T) {
	t.Parallel()

	const src = `bazel_dep(name = "a", version = "1\x2e0")`
	tree := fstest.MapFS{"MODULE.bazel": {Data: []byte(src)}}
	edit, err := workspace.PreviewVersion(t.Context(), tree, "a", "1.0")
	require.NoError(t, err)
	require.False(t, edit.Changed)
	require.Equal(t, "1.0", edit.CurrentVersion)
	require.Equal(t, src, string(edit.Before))
	require.Equal(t, src, string(edit.After))
	edit.After[0] = '!'
	require.Equal(t, src, string(edit.Before))
}

func TestPreviewVersionPreservesRawLiteralDelimiters(t *testing.T) {
	t.Parallel()

	for _, quote := range []string{"'", "'''"} {
		src := "bazel_dep(name = 'a', version = r" + quote + "1.0.0" + quote + ")"
		tree := fstest.MapFS{"MODULE.bazel": {Data: []byte(src)}}
		edit, err := workspace.PreviewVersion(t.Context(), tree, "a", "2.10.0")
		require.NoError(t, err)
		require.True(t, edit.Changed)
		require.Equal(t, strings.Replace(src, "1.0.0", "2.10.0", 1), string(edit.After))
	}
}

func TestPreviewVersionErrors(t *testing.T) {
	t.Parallel()

	const dep = "bazel_dep(name = \"a\", version = \"1\")\n"
	tests := []struct {
		name    string
		root    string
		include string
		module  string
		version string
		want    error
	}{
		{name: "missing module", root: "# empty", module: "a", version: "2", want: workspace.ErrDependencyNotFound},
		{name: "missing root", module: "a", version: "2", want: fs.ErrNotExist},
		{name: "missing include", root: `include("//:dep.MODULE.bazel")`, module: "a", version: "2", want: fs.ErrNotExist},
		{name: "duplicate calls", root: dep + dep, module: "a", version: "2", want: workspace.ErrAmbiguousDependency},
		{name: "duplicate across files", root: dep + `include("//:dep.MODULE.bazel")`, include: dep, module: "a", version: "2", want: workspace.ErrAmbiguousDependency},
		{name: "duplicate name keywords", root: `bazel_dep(name = "a", name = "b", version = "1")`, module: "a", version: "2", want: workspace.ErrAmbiguousDependency},
		{name: "duplicate version keywords", root: `bazel_dep(name = "a", version = "1", version = ver)`, module: "a", version: "2", want: workspace.ErrAmbiguousDependency},
		{name: "absent version", root: `bazel_dep(name = "a")`, module: "a", version: "2", want: workspace.ErrVersionlessDependency},
		{name: "empty version", root: `bazel_dep(name = "a", version = "")`, module: "a", version: "2", want: workspace.ErrVersionlessDependency},
		{name: "nonliteral version", root: `bazel_dep(name = "a", version = ver)`, module: "a", version: "2", want: workspace.ErrVersionlessDependency},
		{name: "invalid name", module: "A", version: "2", want: workspace.ErrInvalidModuleName},
		{name: "invalid version", module: "a", version: "2..0", want: workspace.ErrInvalidVersion},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := fstest.MapFS{}
			if tc.root != "" {
				tree["MODULE.bazel"] = &fstest.MapFile{Data: []byte(tc.root)}
			}
			if tc.include != "" {
				tree["dep.MODULE.bazel"] = &fstest.MapFile{Data: []byte(tc.include)}
			}
			edit, err := workspace.PreviewVersion(t.Context(), tree, tc.module, tc.version)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, workspace.Edit{}, edit)
		})
	}
}

func TestPreviewVersionParseFailure(t *testing.T) {
	t.Parallel()

	tree := fstest.MapFS{"MODULE.bazel": {Data: []byte("bazel_dep(")}}
	edit, err := workspace.PreviewVersion(t.Context(), tree, "a", "2")
	require.ErrorContains(t, err, "MODULE.bazel")
	require.ErrorContains(t, err, "syntax error")
	require.Equal(t, workspace.Edit{}, edit)
}

func TestPreviewVersionCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	tree := &trackingFS{
		MapFS:      fstest.MapFS{"MODULE.bazel": {Data: []byte("bazel_dep(name = \"a\", version = \"1\")\n")}},
		opens:      make(map[string]int),
		beforeOpen: cancel,
	}
	edit, err := workspace.PreviewVersion(ctx, tree, "a", "2")
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, workspace.Edit{}, edit)
	require.Equal(t, 1, tree.opens["MODULE.bazel"])

	edit, err = workspace.PreviewVersion(ctx, tree, "a", "2")
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, workspace.Edit{}, edit)
	require.Equal(t, 1, tree.opens["MODULE.bazel"], "a canceled call may not open another file")
}
