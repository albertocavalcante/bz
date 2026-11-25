package mod

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/module"
)

func writeDepsTable(w io.Writer, f *module.File) error {
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
	// Show file names when an include contributes statements to the module.
	showFile := f.HasIncludedStatements()
	if showFile {
		fmt.Fprintln(tw, "NAME\tVERSION\tDEV\tFILE")
	} else {
		fmt.Fprintln(tw, "NAME\tVERSION\tDEV")
	}

	for _, dep := range f.Deps {
		dev := ""
		if dep.DevDependency {
			dev = cli.Muted("yes")
		}
		if showFile {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", dep.Name.String(), dep.Version.String(), dev, f.OriginOf(dep).File)
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", dep.Name.String(), dep.Version.String(), dev)
	}

	return tw.Flush()
}

func writeFullTable(w io.Writer, f *module.File) error {
	summary := f.ToSummary()
	if summary.Name != "" {
		fmt.Fprintf(w, "%s", summary.Name)
		if summary.Version != "" {
			fmt.Fprintf(w, " (%s)", summary.Version)
		}
		fmt.Fprintln(w)
	}

	// Dependencies
	if len(summary.Deps) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Dependencies:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, dep := range summary.Deps {
			dev := ""
			if dep.Dev {
				dev = cli.Muted(" (dev)")
			}
			fmt.Fprintf(tw, "  %s\t%s%s\n", dep.Name, dep.Version, dev+includedFileSuffix(dep.DeclaredIn))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	// Extensions
	if len(summary.Extensions) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Extensions:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, ext := range summary.Extensions {
			dev := ""
			if ext.DevDependency {
				dev = " (dev)"
			}
			fmt.Fprintf(tw, "  %s\t%s%s\n", ext.Name, ext.File, dev+includedFileSuffix(ext.DeclaredIn))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	// Overrides
	if len(summary.Overrides) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Overrides:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, ov := range summary.Overrides {
			detail := ""
			if ov.Detail != "" {
				detail = " → " + ov.Detail
			}
			fmt.Fprintf(tw, "  %s\t[%s]%s%s\n", ov.Module, ov.Type, detail, includedFileSuffix(ov.DeclaredIn))
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

	if len(summary.Deps) == 0 && len(summary.Extensions) == 0 && len(summary.Overrides) == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "No dependencies, extensions, or overrides found.")
	}

	return nil
}

func includedFileSuffix(file string) string {
	if file == "" {
		return ""
	}
	return " [" + file + "]"
}
