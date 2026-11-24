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
	// Other tests pin doctorCmd's writer to os.Stdout; let it inherit from root.
	doctorCmd.SetOut(nil)
	doctorCmd.SetErr(nil)
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
		// wantErrLines is the expected number of "Error:" lines on stderr.
		wantErrLines int
		// onceSub, when set, must appear exactly once in stdout+stderr combined.
		onceSub string
	}{
		{name: "missing MODULE.bazel", args: []string{"mod", "list"}, wantSub: "MODULE.bazel", wantErrLines: 1},
		{name: "unknown subcommand flag", args: []string{"mod", "sync", "--bogus", "x"}, wantSub: "unknown flag: --bogus", wantErrLines: 1},
		{name: "unknown subcommand", args: []string{"nosuchcmd"}, wantSub: "unknown command", wantErrLines: 1},
		{name: "disabled command", args: []string{"registry", "ping"}, env: map[string]string{"BZ_DISABLE_COMMANDS": "ping"}, wantSub: "disabled", wantErrLines: 1},
		{name: "mutually exclusive offline flags", args: []string{"--offline", "--prefer-offline", "mod", "list"}, wantSub: "offline", wantErrLines: 1},
		{name: "init over existing MODULE.bazel", args: []string{"init"}, module: true, wantSub: "MODULE.bazel", wantErrLines: 1},
		{name: "sbom invalid format", args: []string{"sbom", "--format=xml"}, module: true, wantSub: "unknown format", wantErrLines: 1},
		{name: "unknown flag gets a usage hint", args: []string{"mod", "sync", "--bogus", "x"}, wantSub: "Run 'bz mod sync --help' for usage.", wantErrLines: 1},
		{name: "doctor reports issues once without Error line", args: []string{"--offline", "doctor"}, wantSub: "issue", wantErrLines: 0, onceSub: "issues found"},
		{name: "cache download failure summary once", args: []string{"cache", "download", "--registry", "http://127.0.0.1:1", "--cache-dir", "CACHEDIR", "rules_go@0.50.1"}, wantSub: "1 of 1 module(s) failed to download", wantErrLines: 0, onceSub: "failed to download"},
		{name: "offline cache miss", args: []string{"--offline", "mod", "info", "rules_go"}, wantSub: "offline", wantErrLines: 1},
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

			args := append([]string(nil), tt.args...)
			for i, a := range args {
				if a == "CACHEDIR" {
					args[i] = filepath.Join(dir, "cache")
				}
			}
			stdout, stderr, code := runCLI(t, args...)

			if code == 0 {
				t.Errorf("exit code = 0, want non-zero (stderr=%q)", stderr)
			}
			if strings.TrimSpace(stdout+stderr) == "" || (tt.wantErrLines > 0 && strings.TrimSpace(stderr) == "") {
				t.Fatalf("error was swallowed (stdout=%q stderr=%q)", stdout, stderr)
			}
			if n := countErrorLines(stderr); n != tt.wantErrLines {
				t.Errorf("stderr has %d \"Error:\" lines, want %d:\n%s", n, tt.wantErrLines, stderr)
			}
			if tt.onceSub != "" {
				if n := strings.Count(stdout+stderr, tt.onceSub); n != 1 {
					t.Errorf("%q appears %d times, want 1:\nstdout=%s\nstderr=%s", tt.onceSub, n, stdout, stderr)
				}
			}
			if !strings.Contains(strings.ToLower(stdout+stderr), strings.ToLower(tt.wantSub)) {
				t.Errorf("output missing %q:\nstdout=%s\nstderr=%s", tt.wantSub, stdout, stderr)
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
