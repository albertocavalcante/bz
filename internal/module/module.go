// Package module provides a unified data layer for MODULE.bazel files.
// It wraps the go-bzlmod-ast package with convenient accessors.
package module

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	ast "github.com/albertocavalcante/go-bzlmod-ast"
)

const moduleFileName = "MODULE.bazel"

// ErrModuleFileNotFound indicates MODULE.bazel was not found in current or parent directories.
var ErrModuleFileNotFound = errors.New("MODULE.bazel not found")

// File represents a parsed MODULE.bazel with all its contents.
type File struct {
	Path string

	// Module declaration (may be nil for files without module())
	Module *ast.ModuleDecl

	// Dependencies
	Deps []*ast.BazelDep

	// Extensions
	Extensions []*ast.UseExtension
	ExtTags    []*ast.ExtensionTagCall

	// Overrides
	Overrides []ast.Override

	// Toolchains and platforms
	Toolchains []*ast.RegisterToolchains
	Platforms  []*ast.RegisterExecutionPlatforms

	// Advanced (Bazel 7.2+/8+)
	Includes     []*ast.Include
	UseRepoRules []*ast.UseRepoRule
	FlagAliases  []*ast.FlagAlias

	// Raw AST for advanced use cases
	raw *ast.ModuleFile

	// origins records where statements that came from included files were
	// declared. Statements of the root file have no entry.
	origins map[ast.Statement]Origin
}

// Origin says where a statement was declared.
type Origin struct {
	// File is the declaring file, relative to the workspace root and with
	// forward slashes on every OS ("MODULE.bazel" for the root file).
	File string
	// Pos is the statement's position in File.
	Pos ast.Span
	// Chain lists the include() calls that led to File, outermost first. It
	// is empty for the root file.
	Chain []ast.IncludeSite
}

// IsRoot reports whether the statement lives in the root MODULE.bazel.
func (o Origin) IsRoot() bool { return len(o.Chain) == 0 }

// OriginOf returns where stmt was declared. Statements that were not loaded
// from an included file are reported as declared in the root file.
func (f *File) OriginOf(stmt ast.Statement) Origin {
	if o, ok := f.origins[stmt]; ok {
		return o
	}
	return Origin{File: moduleFileName, Pos: stmt.Span()}
}

// fromSuffix returns " [file]" for statements declared in an included file
// and the empty string for the root file.
func (f *File) fromSuffix(stmt ast.Statement) string {
	if o := f.OriginOf(stmt); !o.IsRoot() {
		return " [" + o.File + "]"
	}
	return ""
}

// Raw returns the underlying AST for advanced operations.
func (f *File) Raw() *ast.ModuleFile {
	return f.raw
}

// Name returns the module name, or empty string if not declared.
func (f *File) Name() string {
	if f.Module == nil {
		return ""
	}
	return f.Module.Name.String()
}

// Version returns the module version, or empty string if not declared.
func (f *File) Version() string {
	if f.Module == nil {
		return ""
	}
	return f.Module.Version.String()
}

// HasOverrides returns true if any overrides are defined.
func (f *File) HasOverrides() bool {
	return len(f.Overrides) > 0
}

// HasExtensions returns true if any extensions are used.
func (f *File) HasExtensions() bool {
	return len(f.Extensions) > 0
}

// Load parses a MODULE.bazel file and returns structured data. include()
// calls of the root module are followed within the directory of path, so Deps
// and Overrides also hold what included files declare, in evaluation order.
// Any include failure fails the load.
func Load(path string) (*File, error) {
	result, err := ast.ParseFile(path)
	f, err := loadParsed(path, result, err)
	if err != nil || len(f.Includes) == 0 {
		return f, err
	}
	return expandIncludes(path, result)
}

// expandIncludes follows the include() calls of the root file. The file system
// is rooted at the workspace so symlinks cannot leave it.
func expandIncludes(path string, root *ast.ParseResult) (*File, error) {
	dir, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open workspace root: %w", err)
	}
	defer dir.Close()

	x, err := ast.ExpandIncludes(context.Background(), root, dir.FS(), ast.IncludeOptions{})
	if err != nil {
		return nil, includeError(err)
	}
	if x.HasErrors() {
		return nil, parseErrorf(x.Errors[0])
	}
	return fromExpanded(path, x), nil
}

// includeError names the include() call when the error is positioned elsewhere,
// as for a statement that is not allowed in an included file.
func includeError(err error) error {
	var ie *ast.IncludeError
	if errors.As(err, &ie) && errors.Is(err, ast.ErrIncludeForbiddenStatement) && len(ie.Chain) > 0 {
		site := ie.Chain[len(ie.Chain)-1]
		return fmt.Errorf("%w (included at %s:%d:%d)", err, site.Pos.Start.Filename, site.Pos.Start.Line, site.Pos.Start.Column)
	}
	return err
}

func parseErrorf(e *ast.ParseError) error {
	return fmt.Errorf("%s:%d:%d: %s", e.Pos.Filename, e.Pos.Line, e.Pos.Column, e.Message)
}

// LoadContent parses MODULE.bazel content from bytes. It does not follow
// include(): there is no file system to read included files from.
func LoadContent(filename string, content []byte) (*File, error) {
	result, err := ast.ParseContent(filename, content)
	return loadParsed(filename, result, err)
}

func loadParsed(path string, result *ast.ParseResult, err error) (*File, error) {
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if result.HasErrors() {
		return nil, parseErrorf(result.Errors[0])
	}

	return fromAST(path, result.File), nil
}

// Find locates MODULE.bazel by walking up from the current directory.
func Find() (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findFrom(startDir)
}

func findFrom(startDir string) (string, error) {
	dir := startDir
	for {
		path := filepath.Join(dir, moduleFileName)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("%w in %s or any parent directory", ErrModuleFileNotFound, startDir)
}

// FindAndLoad finds and loads the MODULE.bazel file.
func FindAndLoad() (*File, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return findAndLoadFrom(startDir)
}

func findAndLoadFrom(startDir string) (*File, error) {
	path, err := findFrom(startDir)
	if err != nil {
		return nil, err
	}
	return Load(path)
}

// fromAST converts the raw AST into our structured File type.
func fromAST(path string, m *ast.ModuleFile) *File {
	f := &File{
		Path: path,
		raw:  m,
	}
	for _, stmt := range m.Statements {
		f.add(stmt)
	}
	return f
}

// fromExpanded builds a File from the root file and everything it includes.
func fromExpanded(path string, x *ast.ExpandedFile) *File {
	root := x.Files[0]
	f := &File{
		Path:    path,
		raw:     root,
		origins: map[ast.Statement]Origin{},
	}
	for _, es := range x.Statements {
		if es.File == root {
			f.add(es.Statement)
			continue
		}
		if _, nested := es.Statement.(*ast.Include); nested {
			continue
		}
		f.origins[es.Statement] = Origin{File: es.File.Path, Pos: es.Span(), Chain: es.Chain}
		f.add(es.Statement)
	}
	return f
}

func (f *File) add(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.ModuleDecl:
		f.Module = s
	case *ast.BazelDep:
		f.Deps = append(f.Deps, s)
	case *ast.UseExtension:
		f.Extensions = append(f.Extensions, s)
	case *ast.ExtensionTagCall:
		f.ExtTags = append(f.ExtTags, s)
	case *ast.SingleVersionOverride:
		f.Overrides = append(f.Overrides, s)
	case *ast.MultipleVersionOverride:
		f.Overrides = append(f.Overrides, s)
	case *ast.GitOverride:
		f.Overrides = append(f.Overrides, s)
	case *ast.ArchiveOverride:
		f.Overrides = append(f.Overrides, s)
	case *ast.LocalPathOverride:
		f.Overrides = append(f.Overrides, s)
	case *ast.RegisterToolchains:
		f.Toolchains = append(f.Toolchains, s)
	case *ast.RegisterExecutionPlatforms:
		f.Platforms = append(f.Platforms, s)
	case *ast.Include:
		f.Includes = append(f.Includes, s)
	case *ast.UseRepoRule:
		f.UseRepoRules = append(f.UseRepoRules, s)
	case *ast.FlagAlias:
		f.FlagAliases = append(f.FlagAliases, s)
	}
}
