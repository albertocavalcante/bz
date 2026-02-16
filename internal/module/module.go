// Package module provides a unified data layer for MODULE.bazel files.
// It wraps go-bzlmod's ast package with convenient accessors.
package module

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/albertocavalcante/go-bzlmod/ast"
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

// Load parses a MODULE.bazel file and returns structured data.
func Load(path string) (*File, error) {
	result, err := ast.ParseFile(path)
	return loadParsed(path, result, err)
}

// LoadContent parses MODULE.bazel content from bytes.
func LoadContent(filename string, content []byte) (*File, error) {
	result, err := ast.ParseContent(filename, content)
	return loadParsed(filename, result, err)
}

func loadParsed(path string, result *ast.ParseResult, err error) (*File, error) {
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if result.HasErrors() {
		e := result.Errors[0]
		return nil, fmt.Errorf("%s:%d:%d: %s", e.Pos.Filename, e.Pos.Line, e.Pos.Column, e.Message)
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

	return f
}
