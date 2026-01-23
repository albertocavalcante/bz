package module

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/albertocavalcante/go-bzlmod/ast"
)

// Dep is a simplified dependency for output.
type Dep struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Dev     bool   `json:"dev_dependency,omitempty"`
}

// Extension is a simplified extension for output.
type Extension struct {
	Name          string `json:"name"`
	File          string `json:"file"`
	DevDependency bool   `json:"dev_dependency,omitempty"`
	Isolate       bool   `json:"isolate,omitempty"`
}

// Override is a simplified override for output.
type Override struct {
	Type   string `json:"type"`
	Module string `json:"module"`
	Detail string `json:"detail,omitempty"`
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
		})
	}

	for _, e := range f.Extensions {
		s.Extensions = append(s.Extensions, Extension{
			Name:          e.ExtensionName.String(),
			File:          e.ExtensionFile.String(),
			DevDependency: e.DevDependency,
			Isolate:       e.Isolate,
		})
	}

	for _, o := range f.Overrides {
		s.Overrides = append(s.Overrides, overrideToSummary(o))
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

// WriteDepsTable writes dependencies as a table.
func (f *File) WriteDepsTable(w io.Writer) error {
	if f.Name() != "" {
		fmt.Fprintf(w, "%s", f.Name())
		if f.Version() != "" {
			fmt.Fprintf(w, " (%s)", f.Version())
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w)
	}

	if len(f.Deps) == 0 {
		fmt.Fprintln(w, "No dependencies found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVERSION\tDEV")

	for _, dep := range f.Deps {
		dev := ""
		if dep.DevDependency {
			dev = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", dep.Name.String(), dep.Version.String(), dev)
	}

	return tw.Flush()
}

// WriteFullTable writes all MODULE.bazel contents as tables.
func (f *File) WriteFullTable(w io.Writer) error {
	if f.Name() != "" {
		fmt.Fprintf(w, "%s", f.Name())
		if f.Version() != "" {
			fmt.Fprintf(w, " (%s)", f.Version())
		}
		fmt.Fprintln(w)
	}

	// Dependencies
	if len(f.Deps) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Dependencies:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, dep := range f.Deps {
			dev := ""
			if dep.DevDependency {
				dev = " (dev)"
			}
			fmt.Fprintf(tw, "  %s\t%s%s\n", dep.Name.String(), dep.Version.String(), dev)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	// Extensions
	if len(f.Extensions) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Extensions:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, ext := range f.Extensions {
			dev := ""
			if ext.DevDependency {
				dev = " (dev)"
			}
			fmt.Fprintf(tw, "  %s\t%s%s\n", ext.ExtensionName.String(), ext.ExtensionFile.String(), dev)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	// Overrides
	if len(f.Overrides) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Overrides:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, o := range f.Overrides {
			ov := overrideToSummary(o)
			detail := ""
			if ov.Detail != "" {
				detail = " → " + ov.Detail
			}
			fmt.Fprintf(tw, "  %s\t[%s]%s\n", ov.Module, ov.Type, detail)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	// Toolchains
	if len(f.Toolchains) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Toolchains:")
		for _, tc := range f.Toolchains {
			for _, p := range tc.Patterns {
				fmt.Fprintf(w, "  %s\n", p)
			}
		}
	}

	// Platforms
	if len(f.Platforms) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Execution Platforms:")
		for _, ep := range f.Platforms {
			for _, p := range ep.Patterns {
				fmt.Fprintf(w, "  %s\n", p)
			}
		}
	}

	if len(f.Deps) == 0 && len(f.Extensions) == 0 && len(f.Overrides) == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "No dependencies, extensions, or overrides found.")
	}

	return nil
}
