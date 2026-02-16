package module

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadContent(t *testing.T) {
	t.Parallel()

	t.Run("parses module declaration", func(t *testing.T) {
		t.Parallel()
		content := `module(name = "test_module", version = "1.0.0")`
		f, err := LoadContent("MODULE.bazel", []byte(content))
		if err != nil {
			t.Fatalf("LoadContent failed: %v", err)
		}
		if f.Name() != "test_module" {
			t.Errorf("expected name 'test_module', got %q", f.Name())
		}
		if f.Version() != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %q", f.Version())
		}
	})

	t.Run("parses dependencies", func(t *testing.T) {
		t.Parallel()
		content := `
module(name = "test", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)
`
		f, err := LoadContent("MODULE.bazel", []byte(content))
		if err != nil {
			t.Fatalf("LoadContent failed: %v", err)
		}
		if len(f.Deps) != 2 {
			t.Fatalf("expected 2 deps, got %d", len(f.Deps))
		}
		if f.Deps[0].Name.String() != "rules_go" {
			t.Errorf("expected first dep 'rules_go', got %q", f.Deps[0].Name.String())
		}
		if !f.Deps[1].DevDependency {
			t.Error("expected gazelle to be a dev dependency")
		}
	})

	t.Run("parses extensions", func(t *testing.T) {
		t.Parallel()
		content := `
module(name = "test", version = "1.0.0")
go = use_extension("@rules_go//go:extensions.bzl", "go")
`
		f, err := LoadContent("MODULE.bazel", []byte(content))
		if err != nil {
			t.Fatalf("LoadContent failed: %v", err)
		}
		if !f.HasExtensions() {
			t.Error("expected HasExtensions() to be true")
		}
		if len(f.Extensions) != 1 {
			t.Fatalf("expected 1 extension, got %d", len(f.Extensions))
		}
	})

	t.Run("parses overrides", func(t *testing.T) {
		t.Parallel()
		content := `
module(name = "test", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
local_path_override(module_name = "rules_go", path = "../rules_go")
git_override(module_name = "gazelle", remote = "https://github.com/test/test.git", commit = "abc123")
`
		f, err := LoadContent("MODULE.bazel", []byte(content))
		if err != nil {
			t.Fatalf("LoadContent failed: %v", err)
		}
		if !f.HasOverrides() {
			t.Error("expected HasOverrides() to be true")
		}
		if len(f.Overrides) != 2 {
			t.Errorf("expected 2 overrides, got %d", len(f.Overrides))
		}
	})

	t.Run("handles empty file", func(t *testing.T) {
		t.Parallel()
		f, err := LoadContent("MODULE.bazel", []byte(""))
		if err != nil {
			t.Fatalf("LoadContent failed: %v", err)
		}
		if f.Name() != "" {
			t.Errorf("expected empty name, got %q", f.Name())
		}
		if len(f.Deps) != 0 {
			t.Errorf("expected 0 deps, got %d", len(f.Deps))
		}
	})
}

func TestWriteDepsTable(t *testing.T) {
	t.Parallel()

	content := `
module(name = "mymod", version = "2.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
`
	f, err := LoadContent("MODULE.bazel", []byte(content))
	if err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}

	var buf bytes.Buffer
	if err := f.WriteDepsTable(&buf); err != nil {
		t.Fatalf("WriteDepsTable failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "mymod") {
		t.Error("expected output to contain module name")
	}
	if !strings.Contains(output, "rules_go") {
		t.Error("expected output to contain rules_go")
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	content := `
module(name = "mymod", version = "1.0.0")
bazel_dep(name = "rules_go", version = "0.50.1")
`
	f, err := LoadContent("MODULE.bazel", []byte(content))
	if err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}

	var buf bytes.Buffer
	if err := f.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"name": "mymod"`) {
		t.Error("expected JSON to contain module name")
	}
	if !strings.Contains(output, `"rules_go"`) {
		t.Error("expected JSON to contain rules_go")
	}
}

func TestFormatBazelDep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modName string
		version string
		dev     bool
		want    string
	}{
		{
			name:    "regular dependency",
			modName: "rules_go",
			version: "0.50.1",
			dev:     false,
			want:    `bazel_dep(name = "rules_go", version = "0.50.1")` + "\n",
		},
		{
			name:    "dev dependency",
			modName: "gazelle",
			version: "0.38.0",
			dev:     true,
			want:    `bazel_dep(name = "gazelle", version = "0.38.0", dev_dependency = True)` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatBazelDep(tt.modName, tt.version, tt.dev)
			if got != tt.want {
				t.Errorf("FormatBazelDep() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadContent_ParseErrorIncludesPosition(t *testing.T) {
	t.Parallel()

	_, err := LoadContent("MODULE.bazel", []byte(`module(name = "broken"`))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}

	errText := err.Error()
	if !strings.Contains(errText, "MODULE.bazel:") {
		t.Fatalf("expected parse error to include file position, got: %q", errText)
	}
}

func TestFindAndLoad(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	modPath := filepath.Join(root, "MODULE.bazel")
	if err := os.WriteFile(modPath, []byte(`module(name = "root_mod", version = "1.2.3")`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	path, err := findFrom(nested)
	if err != nil {
		t.Fatalf("findFrom() error = %v", err)
	}
	if path != modPath {
		t.Fatalf("findFrom() = %q, want %q", path, modPath)
	}

	file, err := findAndLoadFrom(nested)
	if err != nil {
		t.Fatalf("findAndLoadFrom() error = %v", err)
	}
	if file.Path != modPath {
		t.Errorf("findAndLoadFrom().Path = %q, want %q", file.Path, modPath)
	}
	if file.Name() != "root_mod" {
		t.Errorf("findAndLoadFrom().Name() = %q, want %q", file.Name(), "root_mod")
	}
}

func TestFind_PrefersNearestModuleFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	topLevel := filepath.Join(root, "MODULE.bazel")
	if err := os.WriteFile(topLevel, []byte(`module(name = "top", version = "1.0.0")`), 0o600); err != nil {
		t.Fatalf("WriteFile() top-level error = %v", err)
	}

	nearestDir := filepath.Join(root, "workspace")
	if err := os.MkdirAll(nearestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() nearestDir error = %v", err)
	}
	nearest := filepath.Join(nearestDir, "MODULE.bazel")
	if err := os.WriteFile(nearest, []byte(`module(name = "nearest", version = "2.0.0")`), 0o600); err != nil {
		t.Fatalf("WriteFile() nearest error = %v", err)
	}

	nested := filepath.Join(nearestDir, "deep", "path")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() nested error = %v", err)
	}

	got, err := findFrom(nested)
	if err != nil {
		t.Fatalf("findFrom() error = %v", err)
	}
	if got != nearest {
		t.Fatalf("findFrom() = %q, want nearest %q", got, nearest)
	}
}

func TestFind_SkipsDirectoryNamedModuleFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	parentModule := filepath.Join(root, "MODULE.bazel")
	if err := os.WriteFile(parentModule, []byte(`module(name = "parent", version = "1.0.0")`), 0o600); err != nil {
		t.Fatalf("WriteFile() parent module error = %v", err)
	}

	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("MkdirAll() child error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(child, "MODULE.bazel"), 0o755); err != nil {
		t.Fatalf("Mkdir() fake MODULE.bazel dir error = %v", err)
	}

	nested := filepath.Join(child, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() nested error = %v", err)
	}

	got, err := findFrom(nested)
	if err != nil {
		t.Fatalf("findFrom() error = %v", err)
	}
	if got != parentModule {
		t.Fatalf("findFrom() = %q, want %q", got, parentModule)
	}
}

func TestFind_NotFound(t *testing.T) {
	t.Parallel()

	empty := t.TempDir()

	_, err := findFrom(empty)
	if err == nil {
		t.Fatal("expected findFrom() error, got nil")
	}

	if !strings.Contains(err.Error(), "MODULE.bazel not found") {
		t.Fatalf("findFrom() error = %q, want missing file message", err.Error())
	}
}
