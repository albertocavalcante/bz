package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_CacheRejectionsReportedOnce checks that coordinate and consent
// rejections in `cache clear` and `cache download` reach the user exactly
// once, with a clear message, a non-zero exit code, and no filesystem change
// outside the cache.
func TestRun_CacheRejectionsReportedOnce(t *testing.T) {
	tests := []struct {
		name string
		args []string
		// wantErrLines is the expected number of "Error:" lines on stderr.
		wantErrLines int
		// wantStderr must each appear exactly once on stderr.
		wantStderr []string
		// wantStdout must each appear in stdout.
		wantStdout []string
	}{
		{
			name:         "clear parent directory",
			args:         []string{"cache", "clear", "--cache-dir", "CACHEDIR", "-f", ".."},
			wantErrLines: 1,
			wantStderr:   []string{`Error: refusing to clear "..": invalid module name ".."`},
		},
		{
			name:         "clear traversal",
			args:         []string{"cache", "clear", "--cache-dir", "CACHEDIR", "-f", "../../victim"},
			wantErrLines: 1,
			wantStderr:   []string{`Error: refusing to clear "../../victim": invalid module name`},
		},
		{
			name:         "clear empty argument",
			args:         []string{"cache", "clear", "--cache-dir", "CACHEDIR", "-f", ""},
			wantErrLines: 1,
			wantStderr:   []string{`Error: refusing to clear "": invalid module name ""`},
		},
		{
			name:         "clear json without force",
			args:         []string{"cache", "clear", "--cache-dir", "CACHEDIR", "--json"},
			wantErrLines: 1,
			wantStderr:   []string{"Error: refusing to clear 1 module(s) without confirmation: pass --force with --json"},
		},
		{
			name:         "download invalid argument",
			args:         []string{"cache", "download", "--registry", "REGDIR", "--cache-dir", "CACHEDIR", "../../x@1.0"},
			wantErrLines: 1,
			wantStderr:   []string{`Error: invalid module argument "../../x@1.0": invalid module name`},
		},
		{
			// Per-module failures are listed on stdout and summarised once on
			// stderr without an "Error:" line, like other download failures.
			name:         "download all from hostile registry",
			args:         []string{"cache", "download", "--all", "--registry", "REGDIR", "--cache-dir", "CACHEDIR"},
			wantErrLines: 0,
			wantStderr:   []string{"1 of 2 module(s) failed to download"},
			wantStdout:   []string{"Downloaded rules_go@1.0", `"rules_go"@"../../../esc": invalid module version "../../../esc"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			box := filepath.Join(dir, "box")
			cacheDir := filepath.Join(box, "cache")
			regDir := filepath.Join(dir, "reg")
			writeTree(t, dir, map[string]string{
				"box/cache/modules/rules_go/1.0/MODULE.bazel": "cached",
				"box/victim/keep":                       "keep",
				"reg/modules/rules_go/metadata.json":    `{"versions":["1.0","../../../esc"]}`,
				"reg/modules/rules_go/1.0/MODULE.bazel": `module(name = "rules_go", version = "1.0")`,
				// Where the traversal version would read from.
				"esc/MODULE.bazel": "hostile",
			})

			t.Chdir(dir)
			t.Setenv("HOME", dir)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
			t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, ".cache"))
			for _, k := range []string{"BZ_OFFLINE", "BZ_PREFER_OFFLINE", "BZ_REGISTRY", "BZ_DISABLE_COMMANDS"} {
				t.Setenv(k, "")
			}

			args := append([]string(nil), tt.args...)
			for i, a := range args {
				switch a {
				case "CACHEDIR":
					args[i] = cacheDir
				case "REGDIR":
					args[i] = regDir
				}
			}
			stdout, stderr, code := runCLI(t, args...)

			if code != 1 {
				t.Errorf("exit code = %d, want 1 (stderr=%q)", code, stderr)
			}
			if n := countErrorLines(stderr); n != tt.wantErrLines {
				t.Errorf("stderr has %d \"Error:\" lines, want %d:\n%s", n, tt.wantErrLines, stderr)
			}
			for _, want := range tt.wantStderr {
				if n := strings.Count(stderr, want); n != 1 {
					t.Errorf("stderr contains %q %d times, want 1:\n%s", want, n, stderr)
				}
			}
			for _, want := range tt.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout missing %q:\n%s", want, stdout)
				}
			}
			if strings.Contains(stderr, "Usage:") {
				t.Errorf("usage text should be suppressed:\n%s", stderr)
			}

			// Nothing outside the cache changed.
			if _, err := os.Stat(filepath.Join(box, "victim", "keep")); err != nil {
				t.Errorf("file outside the cache was removed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(cacheDir, "modules", "rules_go", "1.0", "MODULE.bazel")); err != nil {
				t.Errorf("cached module was removed: %v", err)
			}
			entries, err := os.ReadDir(box)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 {
				t.Errorf("unexpected entries beside the cache: %v", entries)
			}
		})
	}
}

func writeTree(t *testing.T, base string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
