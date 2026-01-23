package module

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoadContent(t *testing.T) {
	t.Run("parses module declaration", func(t *testing.T) {
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
			got := FormatBazelDep(tt.modName, tt.version, tt.dev)
			if got != tt.want {
				t.Errorf("FormatBazelDep() = %q, want %q", got, tt.want)
			}
		})
	}
}
