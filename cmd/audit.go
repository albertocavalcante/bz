package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/module"
	"github.com/albertocavalcante/bz/internal/osv"
)

var (
	auditJSON      bool
	auditSeverity  string
	auditFix       bool
	auditEcosystem string
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

Modules without a known mapping will be skipped with a warning.
Use --ecosystem to manually specify an ecosystem for all modules.

Returns a non-zero exit code if vulnerabilities are found.

Examples:
  bz audit                        # Scan with automatic ecosystem mapping
  bz audit --json                 # Output as JSON
  bz audit --severity=high        # Only show high/critical vulnerabilities
  bz audit --fix                  # Show suggested updates
  bz audit --ecosystem=Go         # Force all modules to query Go ecosystem`,
	RunE: runAudit,
}

func init() {
	auditCmd.Flags().BoolVar(&auditJSON, "json", false, "Output as JSON")
	auditCmd.Flags().StringVar(&auditSeverity, "severity", "", "Minimum severity to report (critical, high, medium, low)")
	auditCmd.Flags().BoolVar(&auditFix, "fix", false, "Show suggested updates to fix vulnerabilities")
	auditCmd.Flags().StringVar(&auditEcosystem, "ecosystem", "", "OSV ecosystem to query (e.g., Go, PyPI, crates.io, npm, Maven)")
	rootCmd.AddCommand(auditCmd)
}

// AuditResult represents the JSON output structure for audit results.
type AuditResult struct {
	Total           int         `json:"total"`
	Vulnerable      int         `json:"vulnerable"`
	Vulnerabilities []AuditVuln `json:"vulnerabilities"`
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

//nolint:gocyclo // CLI command handler with sequential steps
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

	f, err := module.FindAndLoad()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if len(f.Deps) == 0 {
		fmt.Fprintln(out, "No dependencies found in MODULE.bazel")
		return nil
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Determine minimum severity filter
	minSeverity := osv.SeverityUnknown
	if auditSeverity != "" {
		minSeverity = osv.ParseSeverity(auditSeverity)
	}

	// Collect all vulnerabilities
	var allVulns []AuditVuln
	var skippedModules []string
	var queryErrors []error
	queriedCount := 0

	for _, dep := range f.Deps {
		name := dep.Name.String()
		version := dep.Version.String()

		vulns, err := osvClient.Query(ctx, name, version, auditEcosystem)
		if err != nil {
			// Handle ecosystem mapping errors specially
			var noEcoErr *osv.ErrNoEcosystem
			if errors.As(err, &noEcoErr) {
				skippedModules = append(skippedModules, name)
				continue
			}
			queryErrors = append(queryErrors, fmt.Errorf("%s@%s: %w", name, version, err))
			continue
		}

		queriedCount++

		for i := range vulns {
			v := &vulns[i]
			// Apply severity filter
			if minSeverity != osv.SeverityUnknown && v.Severity.Order() < minSeverity.Order() {
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

	// Report skipped modules (unless JSON output)
	if len(skippedModules) > 0 && !auditJSON {
		fmt.Fprintf(cmd.ErrOrStderr(), "Skipped %d module(s) with no OSV ecosystem mapping:\n", len(skippedModules))
		for _, m := range skippedModules {
			eco, known := osv.MapModuleToEcosystem(m)
			if known && eco == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "  - %s (no OSV equivalent)\n", m)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "  - %s (use --ecosystem to specify)\n", m)
			}
		}
		fmt.Fprintln(cmd.ErrOrStderr())
	}

	// Handle query errors
	if len(queryErrors) > 0 {
		for _, err := range queryErrors {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v\n", err)
		}
		// Only fail if we couldn't query any modules successfully
		if queriedCount == 0 && len(skippedModules) == 0 {
			return fmt.Errorf("failed to query vulnerabilities: %w", queryErrors[0])
		}
	}

	// If all modules were skipped, show a helpful message
	if queriedCount == 0 && len(skippedModules) > 0 {
		fmt.Fprintln(out, "No modules could be checked for vulnerabilities.")
		fmt.Fprintln(out, "Use --ecosystem to specify an OSV ecosystem (e.g., Go, PyPI, npm, Maven)")
		return nil
	}

	// Output results
	if auditJSON {
		if err := printAuditJSON(out, len(f.Deps), allVulns); err != nil {
			return err
		}
	} else {
		if err := printAuditTable(out, allVulns, auditFix); err != nil {
			return err
		}
	}

	// Return error if vulnerabilities found (for non-zero exit code)
	if len(allVulns) > 0 {
		return fmt.Errorf("%d vulnerabilities found", len(allVulns))
	}

	return nil
}

func printAuditJSON(w io.Writer, total int, vulns []AuditVuln) error {
	// Count unique vulnerable modules
	vulnModules := make(map[string]bool)
	for i := range vulns {
		v := &vulns[i]
		vulnModules[v.Module+"@"+v.Version] = true
	}

	result := AuditResult{
		Total:           total,
		Vulnerable:      len(vulnModules),
		Vulnerabilities: vulns,
	}

	if result.Vulnerabilities == nil {
		result.Vulnerabilities = []AuditVuln{}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func printAuditTable(w io.Writer, vulns []AuditVuln, showFix bool) error {
	if len(vulns) == 0 {
		fmt.Fprintln(w, cli.Success("No vulnerabilities found"))
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
