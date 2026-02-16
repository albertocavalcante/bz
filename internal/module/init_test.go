package module

import (
	"os"
	"path/filepath"
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

func TestValidateModuleName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "valid", wantErr: false},
		{name: "my_module", wantErr: false},
		{name: "", wantErr: true},
		{name: "1module", wantErr: true},
		{name: "my Module", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateModuleName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateModuleName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
