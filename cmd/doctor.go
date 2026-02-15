package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/registry"
)

// Status constants for DoctorCheck.
const (
	statusPass    = "pass"
	statusFail    = "fail"
	statusSkipped = "skipped"
)

var (
	doctorJSON bool
)

// defaultDoctorRegistryURL is the default registry to check, can be overridden for testing.
var defaultDoctorRegistryURL = registry.DefaultBCR

// CommandRunner interface for running external commands (allows mocking in tests).
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// RealCommandRunner implements CommandRunner using os/exec.
type RealCommandRunner struct{}

// Run executes a command and returns its output.
func (r *RealCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// commandRunner is the global command runner, can be overridden for testing.
var commandRunner CommandRunner = &RealCommandRunner{}

// DoctorCheck represents the result of a single check.
type DoctorCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"` // "pass" or "fail"
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// DoctorResult represents the complete doctor output.
type DoctorResult struct {
	Checks     []DoctorCheck `json:"checks"`
	IssueCount int           `json:"issue_count"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check Bazel/bzlmod setup and diagnose common issues",
	Long: `Performs health checks on your Bazel/bzlmod setup and provides
suggestions for fixing any issues found.

Checks performed:
  - Bazel installation and version
  - MODULE.bazel file existence
  - .bazelversion file existence
  - Bzlmod enabled status
  - Registry connectivity

Examples:
  bz doctor
  bz doctor --json`,
	RunE:          runDoctor,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	out := cmd.OutOrStdout()

	checks := make([]DoctorCheck, 0, 5)
	var bazelVersion string

	// Print header (non-JSON only)
	if !doctorJSON {
		fmt.Fprintln(out, "Checking Bazel setup...")
		fmt.Fprintln(out)
	}

	// 1. Check Bazel installation
	check, version := checkBazelInstalled(ctx, out)
	bazelVersion = version

	// 2-5. Run remaining checks and combine into one append
	checks = append(checks, check,
		checkModuleBazel(out),
		checkBazelVersion(out),
		checkBzlmodEnabled(out, bazelVersion),
		checkRegistryConnectivity(ctx, out),
	)

	// Count issues
	issueCount := 0
	for i := range checks {
		if checks[i].Status == statusFail {
			issueCount++
		}
	}

	result := DoctorResult{
		Checks:     checks,
		IssueCount: issueCount,
	}

	// Output results
	if doctorJSON {
		return printDoctorJSON(out, result)
	}
	return printDoctorText(out, result)
}

func checkBazelInstalled(ctx context.Context, out io.Writer) (DoctorCheck, string) {
	check := DoctorCheck{
		Name: "bazel_installed",
	}

	output, err := commandRunner.Run(ctx, "bazel", "--version")
	if err != nil {
		check.Status = statusFail
		check.Message = "Bazel not found in PATH"
		check.Suggestion = "Install Bazel from https://bazel.build/install"
		if !doctorJSON {
			printCheckFail(out, check.Message, check.Suggestion)
		}
		return check, ""
	}

	version, err := parseBazelVersion(output)
	if err != nil {
		check.Status = statusFail
		check.Message = "Could not parse Bazel version"
		check.Suggestion = "Run 'bazel --version' manually to check"
		if !doctorJSON {
			printCheckFail(out, check.Message, check.Suggestion)
		}
		return check, ""
	}

	check.Status = statusPass
	check.Message = fmt.Sprintf("Bazel installed (%s)", version)
	if !doctorJSON {
		printCheckPass(out, check.Message)
	}
	return check, version
}

func checkModuleBazel(out io.Writer) DoctorCheck {
	check := DoctorCheck{
		Name: "module_bazel",
	}

	if _, err := os.Stat("MODULE.bazel"); os.IsNotExist(err) {
		check.Status = statusFail
		check.Message = "MODULE.bazel not found"
		check.Suggestion = "Run 'bz init' to create one"
		if !doctorJSON {
			printCheckFail(out, check.Message, check.Suggestion)
		}
		return check
	}

	check.Status = statusPass
	check.Message = "MODULE.bazel found"
	if !doctorJSON {
		printCheckPass(out, check.Message)
	}
	return check
}

func checkBazelVersion(out io.Writer) DoctorCheck {
	check := DoctorCheck{
		Name: "bazelversion_file",
	}

	content, err := os.ReadFile(".bazelversion")
	if os.IsNotExist(err) {
		check.Status = statusFail
		check.Message = ".bazelversion not found"
		check.Suggestion = "Create .bazelversion with your Bazel version"
		if !doctorJSON {
			printCheckFail(out, check.Message, check.Suggestion)
		}
		return check
	}

	if err != nil {
		check.Status = statusFail
		check.Message = "Could not read .bazelversion"
		check.Suggestion = "Check file permissions"
		if !doctorJSON {
			printCheckFail(out, check.Message, check.Suggestion)
		}
		return check
	}

	version := strings.TrimSpace(string(content))
	check.Status = statusPass
	check.Message = fmt.Sprintf(".bazelversion found (%s)", version)
	if !doctorJSON {
		printCheckPass(out, check.Message)
	}
	return check
}

func checkBzlmodEnabled(out io.Writer, bazelVersion string) DoctorCheck {
	check := DoctorCheck{
		Name: "bzlmod_enabled",
	}

	// Check if .bazelrc exists and contains --enable_bzlmod
	bazelrcContent, err := os.ReadFile(".bazelrc")
	if err == nil {
		// Check for --enable_bzlmod in .bazelrc
		if strings.Contains(string(bazelrcContent), "--enable_bzlmod") {
			check.Status = statusPass
			check.Message = "Bzlmod enabled in .bazelrc"
			if !doctorJSON {
				printCheckPass(out, check.Message)
			}
			return check
		}

		// Check for --noenable_bzlmod (explicitly disabled)
		if strings.Contains(string(bazelrcContent), "--noenable_bzlmod") {
			check.Status = statusFail
			check.Message = "Bzlmod explicitly disabled in .bazelrc"
			check.Suggestion = "Remove --noenable_bzlmod from .bazelrc or change to --enable_bzlmod"
			if !doctorJSON {
				printCheckFail(out, check.Message, check.Suggestion)
			}
			return check
		}
	}

	// Bazel 7+ has bzlmod enabled by default
	if isBazel7OrNewer(bazelVersion) {
		check.Status = statusPass
		check.Message = "Bzlmod enabled (default in Bazel 7+)"
		if !doctorJSON {
			printCheckPass(out, check.Message)
		}
		return check
	}

	// Bazel 6.x needs explicit --enable_bzlmod
	if bazelVersion != "" {
		check.Status = statusFail
		check.Message = "Bzlmod not explicitly enabled (Bazel < 7)"
		check.Suggestion = "Add 'common --enable_bzlmod' to .bazelrc"
		if !doctorJSON {
			printCheckFail(out, check.Message, check.Suggestion)
		}
		return check
	}

	// Unknown version, can't determine
	check.Status = statusPass
	check.Message = "Bzlmod enabled (assumed)"
	if !doctorJSON {
		printCheckPass(out, check.Message)
	}
	return check
}

func checkRegistryConnectivity(ctx context.Context, out io.Writer) DoctorCheck {
	check := DoctorCheck{
		Name: "registry_connectivity",
	}

	// Check if offline mode is enabled
	if cli.IsEffectivelyOffline() {
		check.Status = statusSkipped
		check.Message = "Registry connectivity (skipped - offline mode)"
		if !doctorJSON {
			printCheckSkipped(out, check.Message)
		}
		return check
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Ping the registry
	url := defaultDoctorRegistryURL + "/bazel_registry.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		check.Status = statusFail
		check.Message = "Registry unreachable"
		check.Suggestion = "Check your internet connection"
		if !doctorJSON {
			printCheckFail(out, check.Message, check.Suggestion)
		}
		return check
	}

	resp, err := client.Do(req)
	if err != nil {
		check.Status = statusFail
		check.Message = "Registry unreachable"
		check.Suggestion = "Check your internet connection"
		if !doctorJSON {
			printCheckFail(out, check.Message, check.Suggestion)
		}
		return check
	}
	defer resp.Body.Close()

	// Accept 200 OK and 403 Forbidden (some registries block HEAD requests but are still "up")
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusForbidden {
		// Extract hostname from registry URL
		registryHost := strings.TrimPrefix(defaultDoctorRegistryURL, "https://")
		registryHost = strings.TrimPrefix(registryHost, "http://")
		registryHost = strings.Split(registryHost, "/")[0]

		check.Status = statusPass
		check.Message = fmt.Sprintf("Registry reachable (%s)", registryHost)
		if !doctorJSON {
			printCheckPass(out, check.Message)
		}
		return check
	}

	check.Status = statusFail
	check.Message = fmt.Sprintf("Registry returned HTTP %d", resp.StatusCode)
	check.Suggestion = "The registry may be temporarily unavailable"
	if !doctorJSON {
		printCheckFail(out, check.Message, check.Suggestion)
	}
	return check
}

func printCheckPass(out io.Writer, message string) {
	fmt.Fprintf(out, "%s %s\n", passSymbol(), message)
}

func printCheckFail(out io.Writer, message, suggestion string) {
	fmt.Fprintf(out, "%s %s\n", failSymbol(), message)
	if suggestion != "" {
		fmt.Fprintf(out, "  %s %s\n", suggestionArrow(), suggestion)
	}
}

func printCheckSkipped(out io.Writer, message string) {
	fmt.Fprintf(out, "%s %s\n", skippedSymbol(), message)
}

func skippedSymbol() string {
	return cli.Warning("\u2298") // Circled division slash / "skipped" symbol
}

func passSymbol() string {
	return cli.Success("\u2713") // Green checkmark
}

func failSymbol() string {
	return cli.Error("\u2717") // Red X mark
}

func suggestionArrow() string {
	return cli.Info("\u2192") // Blue right arrow
}

func printDoctorText(out io.Writer, result DoctorResult) error {
	fmt.Fprintln(out)
	if result.IssueCount == 0 {
		fmt.Fprintln(out, cli.Success("All checks passed!"))
		return nil
	}

	if result.IssueCount == 1 {
		fmt.Fprintln(out, cli.Warning("1 issue found"))
	} else {
		fmt.Fprintf(out, "%s\n", cli.Warning(fmt.Sprintf("%d issues found", result.IssueCount)))
	}
	return fmt.Errorf("%d issues found", result.IssueCount)
}

func printDoctorJSON(out io.Writer, result DoctorResult) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	if result.IssueCount > 0 {
		return fmt.Errorf("%d issues found", result.IssueCount)
	}
	return nil
}

// parseBazelVersion extracts the version from bazel --version output.
func parseBazelVersion(output string) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", fmt.Errorf("empty output")
	}

	// Try pattern "bazel X.Y.Z" or "Build label: X.Y.Z"
	patterns := []string{
		`bazel\s+(\d+\.\d+\.\d+(?:-[a-zA-Z0-9]+)?)`,
		`Build label:\s*(\d+\.\d+\.\d+(?:-[a-zA-Z0-9]+)?)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(output)
		if len(matches) >= 2 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("could not parse version from output: %s", output)
}

// isBazel7OrNewer returns true if the version is 7.0.0 or newer.
func isBazel7OrNewer(version string) bool {
	if version == "" {
		return false
	}

	// Extract major version
	parts := strings.Split(version, ".")
	if len(parts) < 1 {
		return false
	}

	majorStr := parts[0]
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return false
	}

	return major >= 7
}
