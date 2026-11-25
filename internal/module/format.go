package module

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	ast "github.com/albertocavalcante/go-bzlmod-ast"
)

// Dep is a simplified dependency for output.
type Dep struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Dev     bool   `json:"dev_dependency,omitempty"`
	// DeclaredIn is the included file that declares the dependency; empty
	// for the root MODULE.bazel.
	DeclaredIn string `json:"declared_in,omitempty"`
}

// Extension is a simplified extension for output.
type Extension struct {
	Name          string `json:"name"`
	File          string `json:"file"`
	DevDependency bool   `json:"dev_dependency,omitempty"`
	Isolate       bool   `json:"isolate,omitempty"`
	DeclaredIn    string `json:"declared_in,omitempty"`
}

// Override is a simplified override for output.
type Override struct {
	Type       string `json:"type"`
	Module     string `json:"module"`
	Detail     string `json:"detail,omitempty"`
	DeclaredIn string `json:"declared_in,omitempty"`
}

// Summary is a JSON-serializable summary of a MODULE.bazel file.
type Summary struct {
	Name       string      `json:"name,omitempty"`
	Version    string      `json:"version,omitempty"`
	Deps       []Dep       `json:"dependencies,omitempty"`
	Extensions []Extension `json:"extensions,omitempty"`
	Overrides  []Override  `json:"overrides,omitempty"`
}

// ToSummary converts File to a serializable Summary.
func (f *File) ToSummary() Summary {
	s := Summary{
		Name:    f.Name(),
		Version: f.Version(),
	}

	for _, d := range f.Deps {
		s.Deps = append(s.Deps, Dep{
			Name:    d.Name.String(),
			Version: d.Version.String(),
			Dev:     d.DevDependency,

			DeclaredIn: f.includedFile(d),
		})
	}

	for _, e := range f.Extensions {
		s.Extensions = append(s.Extensions, Extension{
			Name:          e.ExtensionName.String(),
			File:          e.ExtensionFile.String(),
			DevDependency: e.DevDependency,
			Isolate:       e.Isolate,
			DeclaredIn:    f.includedFile(e),
		})
	}

	for _, o := range f.Overrides {
		ov := overrideToSummary(o)
		ov.DeclaredIn = f.includedFile(o)
		s.Overrides = append(s.Overrides, ov)
	}

	return s
}

func overrideToSummary(o ast.Override) Override {
	ov := Override{
		Module: o.ModuleName().String(),
	}

	switch v := o.(type) {
	case *ast.SingleVersionOverride:
		ov.Type = "single_version"
		if v.Version.String() != "" {
			ov.Detail = v.Version.String()
		}
	case *ast.MultipleVersionOverride:
		ov.Type = "multiple_version"
		versions := make([]string, 0, len(v.Versions))
		for _, ver := range v.Versions {
			versions = append(versions, ver.String())
		}
		ov.Detail = strings.Join(versions, ", ")
	case *ast.GitOverride:
		ov.Type = "git"
		if v.Commit != "" {
			ov.Detail = v.Commit[:min(12, len(v.Commit))]
		} else if v.Tag != "" {
			ov.Detail = v.Tag
		} else if v.Branch != "" {
			ov.Detail = v.Branch
		}
	case *ast.ArchiveOverride:
		ov.Type = "archive"
		if len(v.URLs) > 0 {
			ov.Detail = v.URLs[0]
		}
	case *ast.LocalPathOverride:
		ov.Type = "local_path"
		ov.Detail = v.Path
	}

	return ov
}

// FormatBazelDep returns a formatted bazel_dep() statement.
func FormatBazelDep(name, version string, dev bool) string {
	if dev {
		return fmt.Sprintf("bazel_dep(name = %q, version = %q, dev_dependency = True)\n", name, version)
	}
	return fmt.Sprintf("bazel_dep(name = %q, version = %q)\n", name, version)
}

// WriteJSON writes the summary as JSON.
func (f *File) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(f.ToSummary())
}

// includedFile returns the included file that declared stmt, or "" for the
// root file.
func (f *File) includedFile(stmt ast.Statement) string {
	if o := f.OriginOf(stmt); !o.IsRoot() {
		return o.File
	}
	return ""
}
