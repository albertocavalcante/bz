package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/albertocavalcante/bz/internal/cli"
)

// resetCommandTree restores every flag in the command tree to its default so
// consecutive run() calls in one process do not leak state into each other.
func resetCommandTree(c *cobra.Command) {
	reset := func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}
	c.Flags().VisitAll(reset)
	c.PersistentFlags().VisitAll(reset)
	for _, child := range c.Commands() {
		resetCommandTree(child)
	}
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	resetCommandTree(rootCmd)
	cli.Global = cli.Options{}
	cli.ResetCachedConfig()
	t.Cleanup(func() {
		resetCommandTree(rootCmd)
		cli.Global = cli.Options{}
		cli.ResetCachedConfig()
	})

	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

func countErrorLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "Error:") {
			n++
		}
	}
	return n
}

func TestRun_ErrorsArePrintedOnce(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		module  bool // create a MODULE.bazel in the working directory
		wantSub string
	}{
		{name: "missing MODULE.bazel", args: []string{"mod", "list"}, wantSub: "MODULE.bazel"},
		{name: "unknown subcommand flag", args: []string{"mod", "sync", "--workflow", "x"}, wantSub: "unknown flag: --workflow"},
		{name: "unknown subcommand", args: []string{"nosuchcmd"}, wantSub: "unknown command"},
		{name: "disabled command", args: []string{"registry", "ping"}, env: map[string]string{"BZ_DISABLE_COMMANDS": "ping"}, wantSub: "disabled"},
		{name: "mutually exclusive offline flags", args: []string{"--offline", "--prefer-offline", "mod", "list"}, wantSub: "offline"},
		{name: "init over existing MODULE.bazel", args: []string{"init"}, module: true, wantSub: "MODULE.bazel"},
		{name: "sbom invalid format", args: []string{"sbom", "--format=xml"}, module: true, wantSub: "unknown format"},
		{name: "offline cache miss", args: []string{"--offline", "mod", "info", "rules_go"}, wantSub: "offline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.module {
				if err := os.WriteFile(filepath.Join(dir, "MODULE.bazel"), []byte("module(name = \"x\")\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)
			t.Setenv("HOME", dir)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
			for _, k := range []string{"BZ_OFFLINE", "BZ_PREFER_OFFLINE", "BZ_REGISTRY", "BZ_DISABLE_COMMANDS"} {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			stdout, stderr, code := runCLI(t, tt.args...)

			if code == 0 {
				t.Errorf("exit code = 0, want non-zero (stderr=%q)", stderr)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Fatalf("stderr is empty; error was swallowed (stdout=%q)", stdout)
			}
			if n := countErrorLines(stderr); n != 1 {
				t.Errorf("stderr has %d \"Error:\" lines, want exactly 1:\n%s", n, stderr)
			}
			if !strings.Contains(strings.ToLower(stderr), strings.ToLower(tt.wantSub)) {
				t.Errorf("stderr missing %q:\n%s", tt.wantSub, stderr)
			}
			if strings.Contains(stderr, "Usage:") {
				t.Errorf("usage text should be suppressed:\n%s", stderr)
			}
		})
	}
}

func TestRun_Success(t *testing.T) {
	_, stderr, code := runCLI(t, "version")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %q", stderr)
	}
}
