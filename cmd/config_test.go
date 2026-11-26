package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/bzconfig"
	"github.com/albertocavalcante/bz/internal/cli"
)

// configWorkspace creates a workspace directory (with MODULE.bazel), makes
// it the working directory and isolates the user config under a fresh
// XDG_CONFIG_HOME. It returns the workspace and user config file paths.
func configWorkspace(t *testing.T, project, user string) (ws, userFile string) {
	t.Helper()
	ws = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(ws, "MODULE.bazel"), []byte(`module(name = "test", version = "1.0.0")`+"\n"), 0o644))
	if project != "" {
		require.NoError(t, os.WriteFile(filepath.Join(ws, ".bz.toml"), []byte(project), 0o644))
	}
	configHome := t.TempDir()
	userFile = filepath.Join(configHome, "bz", "config.toml")
	if user != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(userFile), 0o755))
		require.NoError(t, os.WriteFile(userFile, []byte(user), 0o644))
	}
	t.Cleanup(cli.SetEnvironment(bzconfig.OSEnvironment().Isolated(configHome)))
	t.Chdir(ws)
	return ws, userFile
}

// TestRun_ConfigFilesAreRead proves the user and project files are loaded
// (earlier releases read only ./.bzconfig.toml).
func TestRun_ConfigFilesAreRead(t *testing.T) {
	t.Run("user file offline mode", func(t *testing.T) {
		configWorkspace(t, "", "[network]\nmode = \"offline\"\n")
		_, stderr, code := runCLI(t, "audit")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "requires network access")
	})
	t.Run("user file disables a command", func(t *testing.T) {
		configWorkspace(t, "", "[commands]\ndisabled = [\"audit\"]\n")
		_, stderr, code := runCLI(t, "audit")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "disabled")
	})
	t.Run("project file found from a subdirectory", func(t *testing.T) {
		ws, _ := configWorkspace(t, "[commands]\ndisabled = [\"audit\"]\n", "")
		sub := filepath.Join(ws, "pkg", "sub")
		require.NoError(t, os.MkdirAll(sub, 0o755))
		t.Chdir(sub)
		_, stderr, code := runCLI(t, "audit")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "disabled")
	})
	t.Run("legacy project file name", func(t *testing.T) {
		ws, _ := configWorkspace(t, "", "")
		require.NoError(t, os.WriteFile(filepath.Join(ws, ".bzconfig.toml"), []byte("[commands]\ndisabled = [\"audit\"]\n"), 0o644))
		_, stderr, code := runCLI(t, "audit")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "disabled")
	})
}

func TestRun_BothProjectFileNamesIsAnError(t *testing.T) {
	ws, _ := configWorkspace(t, "[network]\nmode = \"offline\"\n", "")
	require.NoError(t, os.WriteFile(filepath.Join(ws, ".bzconfig.toml"), []byte(""), 0o644))

	_, stderr, code := runCLI(t, "mod", "list")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "keep only one")
	assert.Equal(t, 1, countErrorLines(stderr))

	// version still works so the problem can be diagnosed.
	_, stderr, code = runCLI(t, "version")
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "Warning:")
}

func TestRun_InvalidConfigValueIsAnError(t *testing.T) {
	_, userFile := configWorkspace(t, "", "[network]\nmode = \"ofline\"\n")

	_, stderr, code := runCLI(t, "mod", "list")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, `invalid network mode "ofline"`)
	assert.Contains(t, stderr, userFile)
	assert.Equal(t, 1, countErrorLines(stderr))
}

// TestRun_ConfigLoadErrorFailsClosed: an invalid value must stop the
// command instead of silently dropping the rest of the configuration
// (here, the disabled list).
func TestRun_ConfigLoadErrorFailsClosed(t *testing.T) {
	configWorkspace(t, "[network]\ntimeout = \"30\"\n\n[commands]\ndisabled = [\"audit\"]\n", "")

	stdout, stderr, code := runCLI(t, "audit")
	assert.Equal(t, 1, code)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `invalid network timeout "30"`)
	assert.Contains(t, stderr, ".bz.toml")
	assert.Equal(t, 1, countErrorLines(stderr), "stderr: %s", stderr)

	// version still runs and shows the problem as a warning.
	_, stderr, code = runCLI(t, "version")
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "Warning:")
	assert.Zero(t, countErrorLines(stderr))
}

// TestRun_FailClosedClasses: every class of configuration failure stops
// the command before it runs, with exactly one Error: line, nothing on
// stdout, the file named and a fix given; version still runs and shows
// the problem as a warning.
func TestRun_FailClosedClasses(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, ws string)
		wantErr []string
	}{
		{
			name: "location error: both project file names",
			setup: func(t *testing.T, ws string) {
				require.NoError(t, os.WriteFile(filepath.Join(ws, ".bz.toml"), nil, 0o600))
				require.NoError(t, os.WriteFile(filepath.Join(ws, ".bzconfig.toml"), nil, 0o600))
			},
			wantErr: []string{".bz.toml", ".bzconfig.toml", "keep only one"},
		},
		{
			name: "untrusted project file",
			setup: func(t *testing.T, ws string) {
				p := filepath.Join(ws, ".bz.toml")
				require.NoError(t, os.WriteFile(p, []byte("[commands]\ndisabled = [\"audit\"]\n"), 0o600))
				require.NoError(t, os.Chmod(p, 0o666))
			},
			wantErr: []string{"untrusted project config", ".bz.toml", "fix: chmod go-w --", "BZ_TRUSTED_CONFIG_DIRS"},
		},
		{
			name: "malformed TOML",
			setup: func(t *testing.T, ws string) {
				require.NoError(t, os.WriteFile(filepath.Join(ws, ".bz.toml"), []byte("[network\n"), 0o600))
			},
			wantErr: []string{"parsing project config", ".bz.toml"},
		},
		{
			name: "invalid value",
			setup: func(t *testing.T, ws string) {
				require.NoError(t, os.WriteFile(filepath.Join(ws, ".bz.toml"), []byte("[network]\nmode = \"ofline\"\n"), 0o600))
			},
			wantErr: []string{`invalid network mode "ofline"`, ".bz.toml", "want"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && strings.HasPrefix(tt.name, "untrusted") {
				t.Skip("no ownership rules on Windows")
			}
			ws, _ := configWorkspace(t, "", "")
			tt.setup(t, ws)

			stdout, stderr, code := runCLI(t, "audit")
			assert.Equal(t, 1, code)
			assert.Empty(t, stdout)
			assert.Equal(t, 1, countErrorLines(stderr), "stderr: %s", stderr)
			for _, sub := range tt.wantErr {
				assert.Contains(t, stderr, sub)
			}

			_, stderr, code = runCLI(t, "version")
			assert.Equal(t, 0, code)
			assert.Zero(t, countErrorLines(stderr), "stderr: %s", stderr)
			assert.Contains(t, stderr, "Warning:")
		})
	}
}

func TestRun_WarningsArePrintedOnce(t *testing.T) {
	configWorkspace(t, "", "")
	t.Setenv("BZ_TRUSTED_CONFIG_DIRS", filepath.Join(t.TempDir(), "missing"))

	_, stderr, code := runCLI(t, "version")
	assert.Equal(t, 0, code)
	assert.Equal(t, 1, strings.Count(stderr, "Warning: BZ_TRUSTED_CONFIG_DIRS entry"), "stderr: %s", stderr)
	assert.Contains(t, stderr, "does not exist; ignored")
}

func TestRun_RegistryCredentialsAreRedacted(t *testing.T) {
	ws, _ := configWorkspace(t, "[network]\nregistry = \"https://user:s3cret@\"\n", "")
	if resolved, err := filepath.EvalSymlinks(ws); err == nil {
		ws = resolved
	}
	stdout, stderr, code := runCLI(t, "audit")
	assert.Equal(t, 1, code)
	assert.Empty(t, stdout)
	assert.Equal(t, "Error: invalid registry in project config "+filepath.Join(ws, ".bz.toml")+`: "https://<redacted>": missing host`+"\n"+
		"help: want https://host[:port]/path, file:///abs/path or an absolute path\n", stderr)
}

// TestRun_DoctorReportsBrokenConfig: doctor runs with a broken config and
// reports it as a failed "config" check, in text and in --json.
func TestRun_DoctorReportsBrokenConfig(t *testing.T) {
	ws, _ := configWorkspace(t, "[network]\nmode = \"ofline\"\n", "")
	if resolved, err := filepath.EvalSymlinks(ws); err == nil {
		ws = resolved
	}
	want := `configuration: invalid network mode "ofline" in project config ` + filepath.Join(ws, ".bz.toml") +
		` (want "online", "prefer-offline" or "offline")`

	// The registry check must do no I/O when the configuration is invalid.
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("doctor contacted the registry despite an invalid configuration: %s %s", r.Method, r.URL)
	}))
	defer registry.Close()
	oldURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = registry.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldURL })

	stdout, stderr, code := runCLI(t, "doctor")
	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, failSymbol()+" "+want+"\n")
	assert.Contains(t, stdout, skippedSymbol()+" Registry connectivity (skipped: configuration is invalid)\n")
	assert.NotContains(t, stderr, "Warning:", "one problem, one message")
	assert.Zero(t, countErrorLines(stderr))

	stdout, _, code = runCLI(t, "doctor", "--json")
	assert.Equal(t, 1, code)
	var result DoctorResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	byName := map[string]DoctorCheck{}
	for _, c := range result.Checks {
		byName[c.Name] = c
	}
	assert.Equal(t, DoctorCheck{Name: "config", Status: statusFail, Message: want}, byName["config"])
	assert.Equal(t, DoctorCheck{Name: "registry_connectivity", Status: statusSkipped,
		Message: "Registry connectivity (skipped: configuration is invalid)"}, byName["registry_connectivity"])
}

// TestRun_SeveralConfigErrorsAreOneList: one Error: header, then one
// indented item per problem.
func TestRun_SeveralConfigErrorsAreOneList(t *testing.T) {
	ws, _ := configWorkspace(t, "[network]\nmode = \"ofline\"\nregistry = \"foo\"\ntimeout = \"30\"\n", "")
	if resolved, err := filepath.EvalSymlinks(ws); err == nil {
		ws = resolved
	}
	src := "project config " + filepath.Join(ws, ".bz.toml")
	stdout, stderr, code := runCLI(t, "mod", "list")
	assert.Equal(t, 1, code)
	assert.Empty(t, stdout)
	assert.Equal(t, "Error: 3 problems:\n"+
		`  - invalid network mode "ofline" in `+src+` (want "online", "prefer-offline" or "offline")`+"\n"+
		`  - invalid registry in `+src+`: "foo": want an https://, http:// or file:// URL or an absolute path`+"\n"+
		`  - invalid network timeout "30" in `+src+` (want a positive Go duration such as "30s")`+"\n", stderr)
}

func TestRun_FlagLayerApplies(t *testing.T) {
	t.Run("an invalid --registry stops the command", func(t *testing.T) {
		configWorkspace(t, "", "")
		// cache stats has no --registry of its own, so the root flag applies.
		stdout, stderr, code := runCLI(t, "--registry", "foo", "cache", "stats")
		assert.Equal(t, 1, code)
		assert.Empty(t, stdout)
		assert.Equal(t, `Error: invalid registry in flag --registry: "foo": want an https://, http:// or file:// URL or an absolute path`+"\n", stderr)
	})
}

// TestRun_HelpOnlyCommandsRunWithBrokenConfig: bare group commands only
// print help, so a broken config is a warning there; commands that act on
// configuration still fail closed.
func TestRun_HelpOnlyCommandsRunWithBrokenConfig(t *testing.T) {
	for _, args := range [][]string{{"mod"}, {"cache"}, {"registry"}, {"help"}, {"version"}, {"completion", "bash"}, {"__complete", "mod", ""}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			configWorkspace(t, "[network\n", "")
			_, stderr, code := runCLI(t, args...)
			assert.Equal(t, 0, code, "stderr: %s", stderr)
			assert.Zero(t, countErrorLines(stderr))
			assert.Contains(t, stderr, "Warning: parsing project config ")
		})
	}
	t.Run("mod list still fails closed", func(t *testing.T) {
		configWorkspace(t, "[network\n", "")
		_, stderr, code := runCLI(t, "mod", "list")
		assert.Equal(t, 1, code)
		assert.Equal(t, 1, countErrorLines(stderr))
	})
}
