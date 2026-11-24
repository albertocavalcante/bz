package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitInDir(t *testing.T) {
	t.Parallel()

	t.Run("creates module file with explicit name and version", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		path, name, err := InitInDir(dir, "my_module", "1.2.3", false)
		if err != nil {
			t.Fatalf("InitInDir() error = %v", err)
		}
		if name != "my_module" {
			t.Fatalf("InitInDir() name = %q, want %q", name, "my_module")
		}
		if path != filepath.Join(dir, "MODULE.bazel") {
			t.Fatalf("InitInDir() path = %q, want %q", path, filepath.Join(dir, "MODULE.bazel"))
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if got := string(content); got != FormatModuleFile("my_module", "1.2.3") {
			t.Fatalf("MODULE.bazel content = %q, want %q", got, FormatModuleFile("my_module", "1.2.3"))
		}
	})

	t.Run("uses sanitized directory name when name is empty", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		dir := filepath.Join(parent, "My Project 123")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}

		path, name, err := InitInDir(dir, "", "", false)
		if err != nil {
			t.Fatalf("InitInDir() error = %v", err)
		}
		if name != "my_project_123" {
			t.Fatalf("InitInDir() name = %q, want %q", name, "my_project_123")
		}
		if path != filepath.Join(dir, "MODULE.bazel") {
			t.Fatalf("InitInDir() path = %q, want %q", path, filepath.Join(dir, "MODULE.bazel"))
		}
	})

	t.Run("errors when file exists and force is false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "MODULE.bazel")
		if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		_, _, err := InitInDir(dir, "my_module", "1.0.0", false)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("overwrites when file exists and force is true", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "MODULE.bazel")
		if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		_, _, err := InitInDir(dir, "my_module", "1.0.0", true)
		if err != nil {
			t.Fatalf("InitInDir() error = %v", err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if got := string(content); got != FormatModuleFile("my_module", "1.0.0") {
			t.Fatalf("MODULE.bazel content = %q, want %q", got, FormatModuleFile("my_module", "1.0.0"))
		}
	})
}

func TestSanitizeModuleName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "my_project", want: "my_project"},
		{input: "MyProject", want: "myproject"},
		{input: "123project", want: "module__123project"},
		{input: "@special#chars", want: "module__special_chars"},
		{input: "", want: "my_module"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := SanitizeModuleName(tt.input); got != tt.want {
				t.Fatalf("SanitizeModuleName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInitInDirRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	const nameRule = `must start with a lowercase letter, end with a lowercase letter or digit, and contain only a-z, 0-9, '.', '-' and '_'`
	const dirNote = " (derived from the directory name)"

	tests := []struct {
		desc, dirName, name, version, want string
	}{
		{"trailing underscore from directory", "my_app_", "", "", `invalid module name "my_app_": ` + nameRule + dirNote},
		{"trailing dash from directory", "proj-", "", "", `invalid module name "proj-": ` + nameRule + dirNote},
		{"trailing dot from directory", "v1.", "", "", `invalid module name "v1.": ` + nameRule + dirNote},
		{"uppercase name", "d", "MyApp", "", `invalid module name "MyApp": ` + nameRule},
		{"invalid version", "d", "app", "not a version", `invalid module version "not a version": must match RELEASE[-PRERELEASE][+BUILD] using only A-Z, a-z, 0-9, '.', '-' and '+'`},
		{"version with newline", "d", "app", "1.0\nFAKE", `invalid module version "1.0\nFAKE": must match RELEASE[-PRERELEASE][+BUILD] using only A-Z, a-z, 0-9, '.', '-' and '+'`},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), tt.dirName)
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}

			_, _, err := InitInDir(dir, tt.name, tt.version, false)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("InitInDir() error = %v, want %q", err, tt.want)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "MODULE.bazel")); !os.IsNotExist(statErr) {
				t.Fatalf("MODULE.bazel was written on refusal (stat error: %v)", statErr)
			}
		})
	}

	t.Run("empty version defaults to 0.0.0", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path, _, err := InitInDir(dir, "app", "", false)
		if err != nil {
			t.Fatalf("InitInDir() error = %v", err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), `version = "0.0.0"`) {
			t.Fatalf("MODULE.bazel = %q, want version 0.0.0", content)
		}
	})

	t.Run("valid name and version", func(t *testing.T) {
		t.Parallel()
		if _, _, err := InitInDir(t.TempDir(), "app", "1.0.0-rc1+build.5", false); err != nil {
			t.Fatalf("InitInDir() error = %v", err)
		}
	})
}
