package module

import (
	"testing"

	ast "github.com/albertocavalcante/go-bzlmod-ast"
	"github.com/albertocavalcante/go-bzlmod-ast/label"
)

func TestToSummary_IncludesOverridesAndExtensions(t *testing.T) {
	t.Parallel()

	extFile, err := label.ParseApparentLabel("@rules_go//go:extensions.bzl")
	if err != nil {
		t.Fatalf("ParseApparentLabel() error = %v", err)
	}
	extName, err := label.NewStarlarkIdentifier("go")
	if err != nil {
		t.Fatalf("NewStarlarkIdentifier() error = %v", err)
	}

	f := &File{
		Module: &ast.ModuleDecl{
			Name:    label.MustModule("demo"),
			Version: label.MustVersion("1.2.3"),
		},
		Deps: []*ast.BazelDep{
			{
				Name:          label.MustModule("rules_go"),
				Version:       label.MustVersion("0.50.0"),
				DevDependency: true,
			},
		},
		Extensions: []*ast.UseExtension{
			{
				ExtensionFile: extFile,
				ExtensionName: extName,
				DevDependency: true,
				Isolate:       true,
			},
		},
		Overrides: []ast.Override{
			&ast.SingleVersionOverride{
				Module:  label.MustModule("single_mod"),
				Version: label.MustVersion("2.0.0"),
			},
			&ast.MultipleVersionOverride{
				Module: label.MustModule("multi_mod"),
				Versions: []label.Version{
					label.MustVersion("1.0.0"),
					label.MustVersion("2.0.0"),
				},
			},
			&ast.GitOverride{
				Module: label.MustModule("git_mod"),
				Commit: "1234567890abcdef",
			},
			&ast.ArchiveOverride{
				Module: label.MustModule("archive_mod"),
				URLs:   []string{"https://example.com/archive.tar.gz"},
			},
			&ast.LocalPathOverride{
				Module: label.MustModule("local_mod"),
				Path:   "../local_mod",
			},
		},
	}

	summary := f.ToSummary()

	if summary.Name != "demo" || summary.Version != "1.2.3" {
		t.Fatalf("summary identity = (%q, %q), want (%q, %q)", summary.Name, summary.Version, "demo", "1.2.3")
	}
	if len(summary.Deps) != 1 {
		t.Fatalf("len(summary.Deps) = %d, want 1", len(summary.Deps))
	}
	if !summary.Deps[0].Dev {
		t.Fatalf("summary dep dev = false, want true")
	}
	if len(summary.Extensions) != 1 {
		t.Fatalf("len(summary.Extensions) = %d, want 1", len(summary.Extensions))
	}
	if !summary.Extensions[0].Isolate || !summary.Extensions[0].DevDependency {
		t.Fatalf("summary extension flags = (%v, %v), want (true, true)", summary.Extensions[0].Isolate, summary.Extensions[0].DevDependency)
	}
	if len(summary.Overrides) != 5 {
		t.Fatalf("len(summary.Overrides) = %d, want 5", len(summary.Overrides))
	}

	byType := map[string]Override{}
	for _, ov := range summary.Overrides {
		byType[ov.Type] = ov
	}

	if got := byType["single_version"].Detail; got != "2.0.0" {
		t.Fatalf("single_version detail = %q, want %q", got, "2.0.0")
	}
	if got := byType["multiple_version"].Detail; got != "1.0.0, 2.0.0" {
		t.Fatalf("multiple_version detail = %q, want %q", got, "1.0.0, 2.0.0")
	}
	if got := byType["git"].Detail; got != "1234567890ab" {
		t.Fatalf("git detail = %q, want %q", got, "1234567890ab")
	}
	if got := byType["archive"].Detail; got != "https://example.com/archive.tar.gz" {
		t.Fatalf("archive detail = %q, want %q", got, "https://example.com/archive.tar.gz")
	}
	if got := byType["local_path"].Detail; got != "../local_mod" {
		t.Fatalf("local_path detail = %q, want %q", got, "../local_mod")
	}
}
