package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/osv"
)

var (
	auditJSON      bool
	auditSeverity  string
	auditFix       bool
	auditEcosystem string

	auditAllowUnchecked bool
)

// osvClient is the OSV API client, can be overridden for testing.
var osvClient osv.Client = osv.NewClient()

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Scan dependencies for security vulnerabilities",
	Long: `Scan all dependencies in MODULE.bazel for known security vulnerabilities
using the Open Source Vulnerabilities (OSV) database.

IMPORTANT: OSV does not have a "Bazel" ecosystem. This command maps known Bazel
modules to their underlying ecosystems:

  rules_go, gazelle     -> Go
  rules_python          -> PyPI
  rules_rust            -> crates.io
  rules_nodejs          -> npm
  rules_java            -> Maven

Only the direct dependencies in MODULE.bazel are audited; transitive
dependencies are not. Most Bazel modules cannot yet be matched to OSV records.
Every run reports "Checked N of M direct dependencies" and lists each one that
was not checked, with the reason. Use --ecosystem to manually specify an ecosystem for all modules.

Exit status is non-zero when vulnerabilities are found, when any OSV query
fails, or when any module could not be checked. --allow-unchecked accepts
modules that have no known mapping; failed queries and vulnerabilities still
fail the run.

Examples:
  bz audit                        # Scan with automatic ecosystem mapping
  bz audit --json                 # Output as JSON
  bz audit --severity=high        # Only show high/critical vulnerabilities
  bz audit --allow-unchecked      # Do not fail on modules with no OSV mapping
  bz audit --fix                  # Show suggested updates
  bz audit --ecosystem=Go         # Force all modules to query Go ecosystem`,
	RunE:          runAudit,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func configureAuditCmd() {
	auditCmd.Flags().BoolVar(&auditJSON, "json", false, "Output as JSON")
	auditCmd.Flags().StringVar(&auditSeverity, "severity", "", "Minimum severity to report (critical, high, medium, low)")
	auditCmd.Flags().BoolVar(&auditFix, "fix", false, "Show suggested updates to fix vulnerabilities")
	auditCmd.Flags().StringVar(&auditEcosystem, "ecosystem", "", "OSV ecosystem to query (e.g., Go, PyPI, crates.io, npm, Maven)")
	auditCmd.Flags().BoolVar(&auditAllowUnchecked, "allow-unchecked", false, "Do not fail when modules could not be checked (failed queries and vulnerabilities still fail)")
	rootCmd.AddCommand(auditCmd)
}

// AuditResult represents the JSON output structure for audit results.
type AuditResult struct {
	Total           int               `json:"total"`
	Vulnerable      int               `json:"vulnerable"`
	Vulnerabilities []AuditVuln       `json:"vulnerabilities"`
	Summary         AuditSummary      `json:"summary"`
	Unchecked       []UncheckedModule `json:"unchecked"`
}

// AuditVuln represents a single vulnerability finding.
type AuditVuln struct {
	Module   string   `json:"module"`
	Version  string   `json:"version"`
	ID       string   `json:"id"`
	Summary  string   `json:"summary"`
	Severity string   `json:"severity"`
	Fixed    string   `json:"fixed,omitempty"`
	Link     string   `json:"link,omitempty"`
	Aliases  []string `json:"aliases,omitempty"`
}

// validSeverities lists the values accepted by --severity.
var validSeverities = []string{"critical", "high", "medium", "low"}

// parseSeverityFlag validates the --severity value. An empty value means no filter.
func parseSeverityFlag(value string) (osv.Severity, error) {
	if value == "" {
		return osv.SeverityUnknown, nil
	}
	sev := osv.ParseSeverity(value)
	if sev == osv.SeverityUnknown {
		return sev, fmt.Errorf("invalid --severity %q: must be one of %s", value, strings.Join(validSeverities, ", "))
	}
	return sev, nil
}

// severityFiltered reports whether a finding is hidden by the threshold.
// A finding whose severity could not be determined is never hidden: it could
// be anything, so a threshold must not silently drop it.
func severityFiltered(found, threshold osv.Severity) bool {
	if threshold == osv.SeverityUnknown || found == osv.SeverityUnknown {
		return false
	}
	return found.Order() < threshold.Order()
}

// auditScope is reported in the JSON summary: transitive dependencies are not audited.
const auditScope = "direct"

// plural returns "1 thing" or "N things".
func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, pluralForm)
}

func deps(n int) string   { return plural(n, "direct dependency", "direct dependencies") }
func vulnsN(n int) string { return plural(n, "vulnerability", "vulnerabilities") }

func wasWere(n int) string {
	if n == 1 {
		return "was"
	}
	return "were"
}

// UncheckedModule is a module for which no vulnerability data was obtained.
type UncheckedModule struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	Reason  string `json:"reason"`
}

// AuditSummary states how much of the dependency set was actually checked.
// Unchecked counts every module without vulnerability data, including the
// Failed ones.
type AuditSummary struct {
	// Scope says what Total counts: only direct dependencies are audited.
	Scope     string `json:"scope"`
	Total     int    `json:"total"`
	Checked   int    `json:"checked"`
	Unchecked int    `json:"unchecked"`
	Failed    int    `json:"failed"`
	// Hidden counts findings below the --severity threshold (text output only).
	Hidden int `json:"-"`
}

func runAudit(cmd *cobra.Command, args []string) error {
	// Check if command is disabled
	if err := cli.CheckCommandAllowed("audit"); err != nil {
		return err
	}

	// Check if offline mode is enabled
	if err := cli.CheckOfflineAllowed("audit"); err != nil {
		var offlineErr *cli.OfflineModeError
		if errors.As(err, &offlineErr) {
			offlineErr.Description = "to query OSV database"
		}
		return err
	}

	minSeverity, err := parseSeverityFlag(auditSeverity)
	if err != nil {
		return err
	}

	f, err := module.FindAndLoad()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if len(f.Deps) == 0 {
		if auditJSON {
			return printAuditJSON(out, nil, nil, AuditSummary{Scope: auditScope})
		}
		fmt.Fprintln(out, "No dependencies found in MODULE.bazel")
		return nil
	}

	ctx := cmdutil.CommandContext(cmd)

	var allVulns []AuditVuln
	var unchecked []UncheckedModule
	summary := AuditSummary{Scope: auditScope, Total: len(f.Deps)}

	for _, dep := range f.Deps {
		name := dep.Name.String()
		version := dep.Version.String()

		vulns, err := osvClient.Query(ctx, name, version, auditEcosystem)
		if err != nil {
			unchecked = append(unchecked, uncheckedFor(name, version, err))
			var noEcoErr *osv.ErrNoEcosystem
			if !errors.As(err, &noEcoErr) {
				summary.Failed++
			}
			continue
		}

		summary.Checked++

		for i := range vulns {
			v := &vulns[i]
			if severityFiltered(v.Severity, minSeverity) {
				summary.Hidden++
				continue
			}

			allVulns = append(allVulns, AuditVuln{
				Module:   name,
				Version:  version,
				ID:       v.ID,
				Summary:  v.Summary,
				Severity: string(v.Severity),
				Fixed:    v.Fixed,
				Link:     v.Link,
				Aliases:  v.Aliases,
			})
		}
	}
	summary.Unchecked = len(unchecked)

	if auditJSON {
		if err := printAuditJSON(out, allVulns, unchecked, summary); err != nil {
			return err
		}
	} else {
		if err := printAuditTable(out, allVulns, auditFix); err != nil {
			return err
		}
		printAuditCoverage(out, allVulns, unchecked, summary, minSeverity)
	}

	return auditVerdict(len(allVulns), summary)
}

// uncheckedFor builds the report entry explaining why a module was not checked.
func uncheckedFor(name, version string, err error) UncheckedModule {
	u := UncheckedModule{Module: name, Version: version}
	var noEcoErr *osv.ErrNoEcosystem
	switch {
	case errors.As(err, &noEcoErr) && noEcoErr.Known:
		u.Reason = "no OSV equivalent for this module"
	case errors.As(err, &noEcoErr):
		u.Reason = "no known OSV ecosystem mapping (use --ecosystem to force one)"
	default:
		u.Reason = "query failed: " + err.Error()
	}
	return u
}

// auditVerdict turns the findings and coverage into the command's exit status.
// Vulnerabilities and failed queries always fail the run; modules with no
// ecosystem mapping fail it unless --allow-unchecked is set.
func auditVerdict(vulns int, s AuditSummary) error {
	var parts []string
	if vulns > 0 {
		parts = append(parts, vulnsN(vulns)+" found")
	}
	if s.Failed > 0 {
		parts = append(parts, fmt.Sprintf("OSV queries failed for %d of %s", s.Failed, deps(s.Total)))
	}
	if n := s.Unchecked - s.Failed; n > 0 && !auditAllowUnchecked {
		parts = append(parts, fmt.Sprintf("%d of %s %s not checked (use --allow-unchecked to accept this)", n, deps(s.Total), wasWere(n)))
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}

// printAuditCoverage states what was and was not checked, so a clean-looking
// result can never be mistaken for full coverage.
func printAuditCoverage(w io.Writer, vulns []AuditVuln, unchecked []UncheckedModule, s AuditSummary, threshold osv.Severity) {
	if len(vulns) > 0 {
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "Checked %d of %s.\n", s.Checked, deps(s.Total))
	if s.Unchecked > 0 {
		fmt.Fprintf(w, "Not checked (%d):\n", s.Unchecked)
		for _, u := range unchecked {
			fmt.Fprintf(w, "  - %s@%s: %s\n", u.Module, u.Version, u.Reason)
		}
	}
	if s.Hidden > 0 {
		fmt.Fprintf(w, "%s below the %s threshold %s not shown.\n", vulnsN(s.Hidden), threshold, wasWere(s.Hidden))
	}
	if len(vulns) > 0 {
		return
	}
	none := "No vulnerabilities found"
	if threshold != osv.SeverityUnknown {
		none = fmt.Sprintf("No vulnerabilities at or above %s found", threshold)
	}
	switch {
	case s.Unchecked == 0:
		fmt.Fprintln(w, cli.Success(none))
	case s.Checked == 0:
		fmt.Fprintln(w, cli.Warning("No direct dependency could be checked; no vulnerability data was obtained."))
	default:
		fmt.Fprintln(w, cli.Warning(fmt.Sprintf(
			"%s in the %s that could be checked; %s %s not checked.",
			none, deps(s.Checked), deps(s.Unchecked), wasWere(s.Unchecked))))
	}
}

func printAuditJSON(w io.Writer, vulns []AuditVuln, unchecked []UncheckedModule, summary AuditSummary) error {
	summary.Scope = auditScope
	// Count unique vulnerable modules
	vulnModules := make(map[string]bool)
	for i := range vulns {
		v := &vulns[i]
		vulnModules[v.Module+"@"+v.Version] = true
	}

	result := AuditResult{
		Total:           summary.Total,
		Vulnerable:      len(vulnModules),
		Vulnerabilities: vulns,
		Summary:         summary,
		Unchecked:       unchecked,
	}
	if result.Unchecked == nil {
		result.Unchecked = []UncheckedModule{}
	}

	if result.Vulnerabilities == nil {
		result.Vulnerabilities = []AuditVuln{}
	}

	return cmdutil.WriteJSON(w, result)
}

func printAuditTable(w io.Writer, vulns []AuditVuln, showFix bool) error {
	if len(vulns) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Header
	header := "Module\tVersion\tID\tSeverity\tSummary"
	separator := "------\t-------\t--\t--------\t-------"
	if showFix {
		header += "\tFixed In"
		separator += "\t--------"
	}
	fmt.Fprintln(tw, header)
	fmt.Fprintln(tw, separator)

	// Group vulnerabilities by module for fix suggestions
	fixSuggestions := make(map[string]string)

	for i := range vulns {
		v := &vulns[i]
		summary := v.Summary
		if len(summary) > 50 {
			summary = summary[:47] + "..."
		}

		severityColored := colorizeSeverity(v.Severity)
		row := fmt.Sprintf("%s\t%s\t%s\t%s\t%s", v.Module, v.Version, v.ID, severityColored, summary)
		if showFix {
			fixVer := v.Fixed
			if fixVer == "" {
				fixVer = "-"
			} else {
				// Track fix suggestions
				key := v.Module + "@" + v.Version
				if existing, ok := fixSuggestions[key]; !ok || fixVer > existing {
					fixSuggestions[key] = fixVer
				}
			}
			row += "\t" + fixVer
		}
		fmt.Fprintln(tw, row)
	}

	if err := tw.Flush(); err != nil {
		return err
	}

	// Print fix suggestions
	if showFix && len(fixSuggestions) > 0 {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Suggested fixes:")
		for key, fixVer := range fixSuggestions {
			parts := strings.Split(key, "@")
			if len(parts) == 2 {
				moduleName := parts[0]
				fmt.Fprintf(w, "  bz mod update %s --version=%s\n", moduleName, fixVer)
			}
		}
	}

	return nil
}

// colorizeSeverity returns the severity string with appropriate color.
func colorizeSeverity(severity string) string {
	switch strings.ToUpper(severity) {
	case "CRITICAL", "HIGH":
		return cli.Error(severity) // Red for critical/high
	case "MEDIUM":
		return cli.Warning(severity) // Yellow for medium
	case "LOW":
		return cli.Info(severity) // Blue for low
	default:
		return severity
	}
}
