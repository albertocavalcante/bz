package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/albertocavalcante/bz/internal/cmdutil"
)

var (
	pingJSON bool
)

// HTTP client configuration for ping
const (
	pingTimeout = 10 * time.Second
)

// Ping status constants.
const (
	pingStatusOK    = "ok"
	pingStatusError = "error"
)

var pingCmd = &cobra.Command{
	Use:   "ping [registry-url]",
	Short: "Check registry availability",
	Long: `Check if a Bazel module registry is available and responding.

Performs a health check by making an HTTP request to the registry
and measuring the response time.

If no registry URL is provided, checks the default Bazel Central Registry (BCR).

Examples:
  bz registry ping                          # Check default BCR
  bz registry ping https://my.registry      # Check specific registry
  bz registry ping --json                   # Output as JSON`,
	Args:          cobra.MaximumNArgs(1),
	RunE:          runPing,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func configurePingCmd() {
	pingCmd.Flags().BoolVar(&pingJSON, "json", false, "Output as JSON")
	Cmd.AddCommand(pingCmd)
}

// pingResult holds the result of a ping operation
type pingResult struct {
	Registry       string `json:"registry"`
	Status         string `json:"status"`
	ResponseTimeMs int64  `json:"response_time_ms"`
	Error          string `json:"error,omitempty"`
}

func runPing(cmd *cobra.Command, args []string) error {
	if err := cli.CheckCommandAllowed("ping"); err != nil {
		return err
	}

	ctx := cmdutil.CommandContext(cmd)

	// Determine registry URL
	registryURL := defaultRegistryURL
	if globalRegistry := cli.GetRegistry(); globalRegistry != "" {
		registryURL = globalRegistry
	}
	if len(args) > 0 {
		registryURL = args[0]
	}

	// Normalize URL - ensure it has a scheme
	registryURL = normalizeURL(registryURL)

	out := cmd.OutOrStdout()

	// Perform the ping
	result := ping(ctx, registryURL)

	if pingJSON {
		return printPingJSON(out, result)
	}
	return printPingText(out, result)
}

// normalizeURL ensures the URL has a proper scheme
func normalizeURL(url string) string {
	// If already has a scheme, return as-is
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return strings.TrimSuffix(url, "/")
	}

	// Add https:// by default
	return "https://" + strings.TrimSuffix(url, "/")
}

// bazelRegistryJSON is the standard marker file for BCR-compatible registries
const bazelRegistryJSON = "bazel_registry.json"

// ping performs the actual health check
func ping(ctx context.Context, registryURL string) pingResult {
	result := pingResult{
		Registry: registryURL,
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: pingTimeout,
	}

	// Use bazel_registry.json as the health check endpoint
	// This is the standard marker file for BCR-compatible registries
	checkURL := registryURL + "/" + bazelRegistryJSON

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, checkURL, nil)
	if err != nil {
		result.Status = pingStatusError
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		return result
	}

	// Measure response time
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)

	result.ResponseTimeMs = elapsed.Milliseconds()

	if err != nil {
		result.Status = pingStatusError
		result.Error = fmt.Sprintf("connection failed: %v", err)
		return result
	}

	// 405 means HEAD is not supported. Retry with GET before deciding status.
	if resp.StatusCode == http.StatusMethodNotAllowed {
		_ = resp.Body.Close()
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
		start = time.Now()
		resp, err = client.Do(req)
		elapsed = time.Since(start)
		result.ResponseTimeMs = elapsed.Milliseconds()

		if err != nil {
			result.Status = pingStatusError
			result.Error = fmt.Sprintf("connection failed: %v", err)
			return result
		}
	}
	defer resp.Body.Close()

	// Check for successful response.
	// Accept 200 OK and 403 Forbidden (some registries block reads but are still "up").
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusForbidden {
		result.Status = pingStatusOK
		return result
	}

	result.Status = pingStatusError
	result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return result
}

func printPingText(w io.Writer, result pingResult) error {
	fmt.Fprintf(w, "Registry: %s\n", result.Registry)

	if result.Status == pingStatusOK {
		fmt.Fprintf(w, "Status: OK\n")
	} else {
		fmt.Fprintf(w, "Status: ERROR\n")
		if result.Error != "" {
			fmt.Fprintf(w, "Error: %s\n", result.Error)
		}
	}

	fmt.Fprintf(w, "Response time: %dms\n", result.ResponseTimeMs)

	// Return error for non-ok status to set exit code
	if result.Status != pingStatusOK {
		return fmt.Errorf("registry unavailable: %s", result.Error)
	}
	return nil
}

func printPingJSON(w io.Writer, result pingResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	// Return error for non-ok status to set exit code
	if result.Status != pingStatusOK {
		return fmt.Errorf("registry unavailable: %s", result.Error)
	}
	return nil
}
