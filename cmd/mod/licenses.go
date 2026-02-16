package mod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/registry"
)

var (
	licensesJSON    bool
	licensesCheck   bool
	licensesSummary bool
	licensesAllow   string
	licensesDeny    string
)

var licensesCmd = &cobra.Command{
	Use:   "licenses",
	Short: "Show license information for dependencies",
	Long: `Display license information for all dependencies in MODULE.bazel.

Fetches license information from registry source.json files when available.
If license information is not available, shows "Unknown".

Use --check with --allow or --deny to verify license compliance.

Examples:
  bz mod licenses                         # List all licenses
  bz mod licenses --json                  # Output as JSON
  bz mod licenses --summary               # Show license summary only
  bz mod licenses --check --deny=GPL-3.0  # Check for denied licenses
  bz mod licenses --check --allow=MIT,Apache-2.0`,
	RunE: runLicenses,
}

func configureLicensesCmd() {
	licensesCmd.Flags().BoolVar(&licensesJSON, "json", false, "Output as JSON")
	licensesCmd.Flags().BoolVar(&licensesCheck, "check", false, "Check licenses against policy")
	licensesCmd.Flags().BoolVar(&licensesSummary, "summary", false, "Show license summary only")
	licensesCmd.Flags().StringVar(&licensesAllow, "allow", "", "Comma-separated list of allowed licenses")
	licensesCmd.Flags().StringVar(&licensesDeny, "deny", "", "Comma-separated list of denied licenses")
	Cmd.AddCommand(licensesCmd)
}

// ModuleLicense holds license information for a module.
type ModuleLicense struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	License string `json:"license"`
}

// LicensesOutput holds the complete licenses output.
type LicensesOutput struct {
	Modules []ModuleLicense `json:"modules"`
	Summary map[string]int  `json:"summary,omitempty"`
}

func runLicenses(cmd *cobra.Command, args []string) error {
	// Check if command is disabled
	if err := cli.CheckCommandAllowed("licenses"); err != nil {
		return err
	}

	// Validate flags
	if licensesCheck && licensesAllow == "" && licensesDeny == "" {
		return fmt.Errorf("--check requires --allow or --deny flag")
	}

	f, err := module.FindAndLoad()
	if err != nil {
		return err
	}

	// Check if there are no dependencies
	if len(f.Deps) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No dependencies found.")
		return err
	}

	ctx := cmdContext(cmd)

	// Create network-aware registry
	reg, err := createNetworkAwareRegistry()
	if err != nil {
		return err
	}

	// Collect license information
	licenses := collectLicenses(ctx, reg, f)

	out := cmd.OutOrStdout()

	// Check mode
	if licensesCheck {
		return checkLicenses(out, licenses)
	}

	// Summary mode
	if licensesSummary {
		return printLicensesSummary(out, licenses)
	}

	// JSON mode
	if licensesJSON {
		return printLicensesJSON(out, licenses)
	}

	// Default table mode
	return printLicensesTable(out, licenses)
}

func collectLicenses(ctx context.Context, reg registry.Registry, f *module.File) []ModuleLicense {
	licenses := make([]ModuleLicense, 0, len(f.Deps))

	for _, dep := range f.Deps {
		license := getLicense(ctx, reg, dep.Name.String(), dep.Version.String())
		licenses = append(licenses, ModuleLicense{
			Name:    dep.Name.String(),
			Version: dep.Version.String(),
			License: license,
		})
	}

	// Sort by module name for consistent output
	sort.Slice(licenses, func(i, j int) bool {
		return licenses[i].Name < licenses[j].Name
	})

	return licenses
}

func getLicense(ctx context.Context, reg registry.Registry, moduleName, version string) string {
	// Try to get source.json to find license info
	sourceData, err := getSourceJSON(ctx, reg, moduleName, version)
	if err != nil || sourceData == nil {
		return "Unknown"
	}

	// Parse source.json and look for license field
	var source map[string]any
	if err := json.Unmarshal(sourceData, &source); err != nil {
		return "Unknown"
	}

	if license, ok := source["license"].(string); ok && license != "" {
		return license
	}

	return "Unknown"
}

// getSourceJSON fetches source.json from the registry.
func getSourceJSON(ctx context.Context, reg registry.Registry, moduleName, version string) ([]byte, error) {
	// Check if registry supports SourceGetter interface
	if sg, ok := reg.(registry.SourceGetter); ok {
		return sg.GetSource(ctx, moduleName, version)
	}

	return nil, nil
}

func printLicensesTable(w io.Writer, licenses []ModuleLicense) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw, "Module\tVersion\tLicense")
	fmt.Fprintln(tw, "------\t-------\t-------")

	for _, lic := range licenses {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", lic.Name, lic.Version, lic.License)
	}

	// Add summary
	fmt.Fprintln(tw)
	summary := buildSummary(licenses)
	fmt.Fprintf(tw, "Summary: %s\n", formatSummary(summary))

	return tw.Flush()
}

func printLicensesJSON(w io.Writer, licenses []ModuleLicense) error {
	output := LicensesOutput{
		Modules: licenses,
		Summary: buildSummary(licenses),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func printLicensesSummary(w io.Writer, licenses []ModuleLicense) error {
	summary := buildSummary(licenses)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw, "License\tCount")
	fmt.Fprintln(tw, "-------\t-----")

	// Sort by count descending, then by name
	type licenseCount struct {
		name  string
		count int
	}
	counts := make([]licenseCount, 0, len(summary))
	for name, count := range summary {
		counts = append(counts, licenseCount{name: name, count: count})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].count != counts[j].count {
			return counts[i].count > counts[j].count
		}
		return counts[i].name < counts[j].name
	})

	for _, lc := range counts {
		fmt.Fprintf(tw, "%s\t%d\n", lc.name, lc.count)
	}

	return tw.Flush()
}

func buildSummary(licenses []ModuleLicense) map[string]int {
	summary := make(map[string]int)
	for _, lic := range licenses {
		summary[lic.License]++
	}
	return summary
}

func formatSummary(summary map[string]int) string {
	// Sort by count descending
	type licenseCount struct {
		name  string
		count int
	}
	counts := make([]licenseCount, 0, len(summary))
	for name, count := range summary {
		counts = append(counts, licenseCount{name: name, count: count})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].count != counts[j].count {
			return counts[i].count > counts[j].count
		}
		return counts[i].name < counts[j].name
	})

	parts := make([]string, 0, len(counts))
	for _, lc := range counts {
		parts = append(parts, fmt.Sprintf("%d %s", lc.count, lc.name))
	}

	return strings.Join(parts, ", ")
}

func checkLicenses(w io.Writer, licenses []ModuleLicense) error {
	var allowList, denyList []string

	if licensesAllow != "" {
		allowList = strings.Split(licensesAllow, ",")
		for i := range allowList {
			allowList[i] = strings.TrimSpace(allowList[i])
		}
	}

	if licensesDeny != "" {
		denyList = strings.Split(licensesDeny, ",")
		for i := range denyList {
			denyList[i] = strings.TrimSpace(denyList[i])
		}
	}

	var violations []ModuleLicense

	for _, lic := range licenses {
		// Check deny list first
		if len(denyList) > 0 {
			for _, denied := range denyList {
				if strings.EqualFold(lic.License, denied) {
					violations = append(violations, lic)
					break
				}
			}
		}

		// Check allow list
		if len(allowList) > 0 {
			allowed := false
			for _, a := range allowList {
				if strings.EqualFold(lic.License, a) {
					allowed = true
					break
				}
			}
			if !allowed {
				// Check if already in violations
				alreadyViolation := false
				for _, v := range violations {
					if v.Name == lic.Name && v.Version == lic.Version {
						alreadyViolation = true
						break
					}
				}
				if !alreadyViolation {
					violations = append(violations, lic)
				}
			}
		}
	}

	if len(violations) == 0 {
		fmt.Fprintln(w, cli.Success("\u2713 All licenses are compliant"))
		return nil
	}

	// Report violations
	var sb strings.Builder
	sb.WriteString(cli.Error("license policy violation:") + "\n")
	for _, v := range violations {
		fmt.Fprintf(&sb, "  %s %s@%s: %s\n", cli.Error("-"), v.Name, v.Version, cli.Warning(v.License))
	}

	return fmt.Errorf("%s", sb.String())
}
