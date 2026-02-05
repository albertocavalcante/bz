package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCommandRunner is used for mocking exec.Command calls
type MockCommandRunner struct {
	Output string
	Error  error
}

func (m *MockCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	return m.Output, m.Error
}

func setupDoctorTest(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	require.NoError(t, os.Chdir(tmpDir))

	return tmpDir
}

func resetDoctorFlags() {
	doctorJSON = false
}

func TestDoctorCmd_BazelInstallationCheck_Success(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel to satisfy that check
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelversion to satisfy that check
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check with a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Bazel installed")
	assert.Contains(t, output, "7.0.0")
}

func TestDoctorCmd_BazelInstallationCheck_NotInstalled(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel to satisfy that check
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Mock bazel command failure
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "",
		Error:  os.ErrNotExist,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check with a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.Error(t, err) // Should return error when issues found

	output := buf.String()
	assert.Contains(t, output, "Bazel not found")
}

func TestDoctorCmd_ModuleBazelCheck_Exists(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelversion to satisfy that check
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MODULE.bazel found")
}

func TestDoctorCmd_ModuleBazelCheck_NotFound(t *testing.T) {
	_ = setupDoctorTest(t)
	resetDoctorFlags()

	// Don't create MODULE.bazel

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.Error(t, err) // Should return error when issues found

	output := buf.String()
	assert.Contains(t, output, "MODULE.bazel not found")
	assert.Contains(t, output, "bz init")
}

func TestDoctorCmd_BazelVersionCheck_Exists(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelversion
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, ".bazelversion found")
	assert.Contains(t, output, "7.0.0")
}

func TestDoctorCmd_BazelVersionCheck_NotFound(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Don't create .bazelversion

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.Error(t, err) // Should return error when issues found

	output := buf.String()
	assert.Contains(t, output, ".bazelversion not found")
}

func TestDoctorCmd_BzlmodEnabledCheck_Enabled(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelversion to satisfy that check
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Create .bazelrc with bzlmod enabled
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelrc"),
		[]byte("common --enable_bzlmod"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Bzlmod enabled")
}

func TestDoctorCmd_BzlmodEnabledCheck_NoBazelrc(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelversion to satisfy that check
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Don't create .bazelrc

	// Mock bazel command (Bazel 7+ has bzlmod enabled by default)
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	// Bazel 7+ has bzlmod enabled by default
	assert.Contains(t, output, "Bzlmod enabled")
}

func TestDoctorCmd_BzlmodEnabledCheck_Bazel6WithoutExplicitEnable(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelrc without explicit bzlmod flag
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelrc"),
		[]byte("build --jobs=8"),
		0o644,
	))

	// Mock bazel command for version 6.x (bzlmod not enabled by default)
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 6.4.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.Error(t, err) // Should return error when issues found

	output := buf.String()
	assert.Contains(t, output, "Bzlmod not explicitly enabled")
}

func TestDoctorCmd_RegistryConnectivityCheck_Reachable(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelversion to satisfy that check
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check with a healthy server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Registry reachable")
}

func TestDoctorCmd_RegistryConnectivityCheck_Unreachable(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelversion to satisfy that check
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Start a server and close it immediately to simulate unreachable
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = serverURL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.Error(t, err) // Should return error when issues found

	output := buf.String()
	assert.Contains(t, output, "Registry unreachable")
}

func TestDoctorCmd_JSONOutput(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()
	doctorJSON = true

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelversion
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	// Parse JSON output
	var result DoctorResult
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Len(t, result.Checks, 5)
	assert.Equal(t, 0, result.IssueCount)
}

func TestDoctorCmd_JSONOutput_WithIssues(t *testing.T) {
	_ = setupDoctorTest(t)
	resetDoctorFlags()
	doctorJSON = true

	// Don't create MODULE.bazel - this will cause an issue

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.Error(t, err) // Should return error when issues found

	// Parse JSON output
	var result DoctorResult
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Greater(t, result.IssueCount, 0)

	// Find the MODULE.bazel check
	var moduleCheck *DoctorCheck
	for i := range result.Checks {
		if result.Checks[i].Name == "module_bazel" {
			moduleCheck = &result.Checks[i]
			break
		}
	}
	require.NotNil(t, moduleCheck)
	assert.Equal(t, "fail", moduleCheck.Status)
	assert.NotEmpty(t, moduleCheck.Suggestion)
}

func TestDoctorCmd_AllChecksPassed(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create all required files
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelrc"),
		[]byte("common --enable_bzlmod"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "All checks passed")
}

func TestDoctorCmd_IssuesFound(t *testing.T) {
	_ = setupDoctorTest(t)
	resetDoctorFlags()

	// Don't create MODULE.bazel or .bazelversion - multiple issues

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issues found")

	output := buf.String()
	assert.Contains(t, output, "issues found")
}

func TestDoctorCmd_ChecksHeader(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create MODULE.bazel
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	// Create .bazelversion to satisfy that check
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Mock registry check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldRegistryURL := defaultDoctorRegistryURL
	defaultDoctorRegistryURL = server.URL
	t.Cleanup(func() { defaultDoctorRegistryURL = oldRegistryURL })

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	_ = doctorCmd.RunE(doctorCmd, []string{})

	output := buf.String()
	assert.Contains(t, output, "Checking Bazel setup")
}

func TestParseBazelVersion(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"bazel 7.0.0", "7.0.0", false},
		{"bazel 6.4.0", "6.4.0", false},
		{"bazel 7.1.0-rc1", "7.1.0-rc1", false},
		{"Build label: 7.0.0", "7.0.0", false},
		{"", "", true},
		{"invalid output", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseBazelVersion(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIsBazel7OrNewer(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"7.0.0", true},
		{"7.1.0", true},
		{"8.0.0", true},
		{"6.4.0", false},
		{"6.0.0", false},
		{"5.4.1", false},
		{"7.0.0-rc1", true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := isBazel7OrNewer(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDoctorCmd_OfflineMode_SkipsRegistryCheck(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()

	// Create required files
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Set offline mode via environment variable
	t.Setenv("BZ_OFFLINE", "1")

	// Don't set up a registry server - it should be skipped in offline mode

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	// Should indicate registry check was skipped
	assert.Contains(t, output, "Registry connectivity")
	assert.Contains(t, output, "skipped")
	assert.Contains(t, output, "offline")
}

func TestDoctorCmd_OfflineMode_JSON_SkipsRegistryCheck(t *testing.T) {
	tmpDir := setupDoctorTest(t)
	resetDoctorFlags()
	doctorJSON = true

	// Create required files
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "MODULE.bazel"),
		[]byte(`module(name = "test", version = "1.0.0")`),
		0o644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".bazelversion"),
		[]byte("7.0.0"),
		0o644,
	))

	// Mock bazel command
	oldRunner := commandRunner
	commandRunner = &MockCommandRunner{
		Output: "bazel 7.0.0",
		Error:  nil,
	}
	t.Cleanup(func() { commandRunner = oldRunner })

	// Set offline mode via environment variable
	t.Setenv("BZ_OFFLINE", "1")

	var buf bytes.Buffer
	doctorCmd.SetOut(&buf)
	t.Cleanup(func() { doctorCmd.SetOut(os.Stdout) })

	err := doctorCmd.RunE(doctorCmd, []string{})
	require.NoError(t, err)

	// Parse JSON output
	var result DoctorResult
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Find the registry check
	var regCheck *DoctorCheck
	for i := range result.Checks {
		if result.Checks[i].Name == "registry_connectivity" {
			regCheck = &result.Checks[i]
			break
		}
	}
	require.NotNil(t, regCheck)
	assert.Equal(t, "skipped", regCheck.Status)
	assert.Contains(t, regCheck.Message, "offline")
}
