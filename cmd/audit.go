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
	auditJSON       bool
	auditSeverity   string
	auditFix        bool
	auditEcosystem  string
	auditUnverified bool

	auditAllowUnchecked bool
)

// osvClient is the OSV API client, can be overridden for testing.
var osvClient osv.Client = osv.NewClient()

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Scan dependencies for security vulnerabilities (no module can be matched yet)",
	Long: `Scan all dependencies in MODULE.bazel for known security vulnerabilities
using the Open Source Vulnerabilities (OSV) database.

OSV files records under upstream package names (for example the Go module
github.com/bazelbuild/rules_go), not under Bazel module names, and bz does not
know which upstream package a module corresponds to. Without --ecosystem, no
module can be checked: each is listed as not checked and the run fails.
--allow-unchecked makes it pass while auditing nothing.

--ecosystem queries that OSV ecosystem for every module, using the Bazel module
name as the package name. It only finds modules whose name equals the upstream
package name, so finding nothing does not show the modules are free of
vulnerabilities; bz says so and prints a note on stderr.

Only the direct dependencies in MODULE.bazel are audited; transitive
dependencies are not. Every run reports "Checked N of M direct dependencies"
and lists each one that was not checked, with the reason.

Exit status is non-zero when vulnerabilities are found, when any OSV query
fails, or when any module could not be checked. --allow-unchecked accepts
modules that have no package mapping; failed queries and vulnerabilities still
fail the run.

Examples:
  bz audit                        # Lists every module as not checked (no mapping yet)
  bz audit --json                 # Output as JSON
  bz audit --severity=high        # Only show high/critical vulnerabilities
  bz audit --allow-unchecked      # Do not fail on modules with no package mapping
  bz audit --fix                  # Show suggested updates
  bz audit --ecosystem=Go         # Query every module as a Go package named after the module`,
	RunE:          runAudit,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func configureAuditCmd() {
	auditCmd.Flags().BoolVar(&auditJSON, "json", false, "Output as JSON")
	auditCmd.Flags().StringVar(&auditSeverity, "severity", "", "Minimum severity to report (critical, high, medium, low)")
	auditCmd.Flags().BoolVar(&auditFix, "fix", false, "Show suggested updates to fix vulnerabilities")
	auditCmd.Flags().StringVar(&auditEcosystem, "ecosystem", "", "OSV ecosystem to query for every module, with the module name as the package name (e.g., Go, PyPI, crates.io, npm, Maven)")
	auditCmd.Flags().BoolVar(&auditUnverified, "ecosystem-unverified", false, "Accept an --ecosystem name that is not in bz's list of OSV ecosystems (a misspelling then matches nothing)")
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
	Scope string `json:"scope"`
	// Ecosystem is set when --ecosystem forced it for every module.
	Ecosystem string `json:"ecosystem,omitempty"`
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

	ecosystem, err := resolveEcosystem(cmd.ErrOrStderr())
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
	summary := AuditSummary{Scope: auditScope, Ecosystem: ecosystem, Total: len(f.Deps)}

	for _, dep := range f.Deps {
		name := dep.Name.String()
		version := dep.Version.String()

		if ecosystem == "" {
			unchecked = append(unchecked, UncheckedModule{Module: name, Version: version, Reason: reasonNoMapping})
			continue
		}

		vulns, err := osvClient.Query(ctx, name, version, ecosystem)
		if err != nil {
			unchecked = append(unchecked, UncheckedModule{Module: name, Version: version, Reason: "query failed: " + err.Error()})
			summary.Failed++
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

	// Exit 0 with a forced ecosystem proves nothing: say so where CI logs show it.
	if ecosystem != "" && len(allVulns) == 0 && summary.Failed == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: --ecosystem=%s queries module names as package names; exit status 0 does not mean the modules were audited\n", ecosystem)
	}

	return auditVerdict(len(allVulns), summary)
}

// resolveEcosystem validates --ecosystem. A name outside bz's list is an error
// unless --ecosystem-unverified opts in, because OSV matches names exactly and
// a typo would otherwise look like a clean result.
func resolveEcosystem(stderr io.Writer) (string, error) {
	if auditEcosystem == "" {
		if auditUnverified {
			return "", errors.New("--ecosystem-unverified needs --ecosystem")
		}
		return "", nil
	}
	eco, err := osv.ParseEcosystem(auditEcosystem)
	if err == nil {
		return eco, nil
	}
	if !auditUnverified {
		return "", fmt.Errorf("invalid --ecosystem: %w", err)
	}
	if !osv.IsSaneEcosystemName(auditEcosystem) {
		return "", fmt.Errorf("invalid --ecosystem %q: not a plausible ecosystem name", osv.Truncate(auditEcosystem, 64))
	}
	fmt.Fprintf(stderr, "warning: --ecosystem=%s is not in bz's list of OSV ecosystems (2026-11-24); a misspelling would silently match nothing\n", auditEcosystem)
	return auditEcosystem, nil
}

// reasonNoMapping explains modules bz cannot query: OSV matches on upstream
// package name (https://ossf.github.io/osv-schema/#affectedpackage-field).
const reasonNoMapping = "no package mapping: bz does not know which upstream package this module corresponds to"

// auditVerdict turns the findings and coverage into the command's exit status.
// Vulnerabilities and failed queries always fail the run; modules with no
// package mapping fail it unless --allow-unchecked is set.
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
// result can never be mistaken for full coverage. With a forced ecosystem the
// module names are only guesses at package names, so it says "queried" and never
// claims the absence of records means the modules are clean.
func printAuditCoverage(w io.Writer, vulns []AuditVuln, unchecked []UncheckedModule, s AuditSummary, threshold osv.Severity) {
	if len(vulns) > 0 {
		fmt.Fprintln(w)
	}
	verb, not := "Checked", "Not checked"
	if s.Ecosystem != "" {
		verb, not = "Queried", "Not queried"
		as := fmt.Sprintf("as %s packages", s.Ecosystem)
		if s.Total == 1 {
			as = fmt.Sprintf("as a %s package", s.Ecosystem)
		}
		fmt.Fprintf(w, "%s %d of %s %s named after the module.\n", verb, s.Checked, deps(s.Total), as)
	} else {
		fmt.Fprintf(w, "%s %d of %s.\n", verb, s.Checked, deps(s.Total))
	}
	switch {
	case s.Unchecked == 0:
	case s.Ecosystem == "":
		// Without an ecosystem every module has the same reason: state it once.
		fmt.Fprintf(w, "%s (%d): %s\n", not, s.Unchecked, reasonNoMapping)
		for _, u := range unchecked {
			fmt.Fprintf(w, "  - %s@%s\n", u.Module, u.Version)
		}
		fmt.Fprintln(w, "help: vulnerability matching for Bazel modules is not available yet; --allow-unchecked accepts that and audits nothing")
	default:
		fmt.Fprintf(w, "%s (%d):\n", not, s.Unchecked)
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
	if s.Ecosystem != "" {
		printForcedNoRecords(w, s, threshold)
		return
	}
	// Without an ecosystem nothing is queried, so there is never a clean result to report.
	fmt.Fprintln(w, cli.Warning("No direct dependency could be checked; no vulnerability data was obtained."))
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

// printForcedNoRecords words the empty result of a forced ecosystem so it only
// claims what was actually queried; failed queries produced no data at all.
func printForcedNoRecords(w io.Writer, s AuditSummary, threshold osv.Severity) {
	if s.Checked == 0 {
		fmt.Fprintln(w, "No direct dependency could be queried; no OSV data was obtained.")
		return
	}
	found := "No OSV records found"
	if threshold != osv.SeverityUnknown {
		found = fmt.Sprintf("No OSV records at or above %s found", threshold)
	}
	if s.Checked == s.Total {
		fmt.Fprintf(w, "%s under these names in %s; this does not show the modules are free of vulnerabilities.\n", found, s.Ecosystem)
		return
	}
	names := "their names"
	if s.Checked == 1 {
		names = "its name"
	}
	fmt.Fprintf(w, "%s for the %s that could be queried, under %s in %s; this does not show the modules are free of vulnerabilities.\n",
		found, deps(s.Checked), names, s.Ecosystem)
}
