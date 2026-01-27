package modules

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

func TestRegistryHTTP(t *testing.T) {
	t.Run("creates HTTP registry with URL", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("https://bcr.bazel.build"),
		}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reg, ok := result.(*RegistryValue)
		if !ok {
			t.Fatalf("expected *RegistryValue, got %T", result)
		}

		if reg.Kind != RegistryTypeHTTP {
			t.Errorf("expected type %q, got %q", RegistryTypeHTTP, reg.Kind)
		}
		if reg.URL != "https://bcr.bazel.build" {
			t.Errorf("expected URL 'https://bcr.bazel.build', got %q", reg.URL)
		}
	})

	t.Run("creates HTTP registry with kwarg", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("https://example.com/registry")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reg := result.(*RegistryValue)
		if reg.URL != "https://example.com/registry" {
			t.Errorf("expected URL 'https://example.com/registry', got %q", reg.URL)
		}
	})

	t.Run("exposes type attribute", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("https://bcr.bazel.build"),
		}, nil)

		reg := result.(*RegistryValue)
		typeAttr, err := reg.Attr("type")
		if err != nil {
			t.Fatalf("failed to get type attr: %v", err)
		}
		if string(typeAttr.(starlark.String)) != RegistryTypeHTTP {
			t.Errorf("type attr mismatch")
		}
	})

	t.Run("exposes url attribute", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("https://bcr.bazel.build"),
		}, nil)

		reg := result.(*RegistryValue)
		urlAttr, err := reg.Attr("url")
		if err != nil {
			t.Fatalf("failed to get url attr: %v", err)
		}
		if string(urlAttr.(starlark.String)) != "https://bcr.bazel.build" {
			t.Errorf("url attr mismatch")
		}
	})

	t.Run("errors on missing URL", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, nil)
		if err == nil {
			t.Fatal("expected error for missing URL")
		}
		if !strings.Contains(err.Error(), "url") {
			t.Errorf("error should mention 'url': %v", err)
		}
	})

	t.Run("String representation", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("https://bcr.bazel.build"),
		}, nil)

		reg := result.(*RegistryValue)
		str := reg.String()
		if !strings.Contains(str, "http") {
			t.Errorf("String() should contain 'http': %q", str)
		}
		if !strings.Contains(str, "bcr.bazel.build") {
			t.Errorf("String() should contain URL: %q", str)
		}
	})
}

func TestRegistryFile(t *testing.T) {
	t.Run("creates file registry with path", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["file"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("/var/cache/bz/registry"),
		}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reg, ok := result.(*RegistryValue)
		if !ok {
			t.Fatalf("expected *RegistryValue, got %T", result)
		}

		if reg.Kind != RegistryTypeFile {
			t.Errorf("expected type %q, got %q", RegistryTypeFile, reg.Kind)
		}
		if reg.URL != "/var/cache/bz/registry" {
			t.Errorf("expected path '/var/cache/bz/registry', got %q", reg.URL)
		}
	})

	t.Run("creates file registry with kwarg", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["file"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("path"), starlark.String("./local-registry")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reg := result.(*RegistryValue)
		if reg.URL != "./local-registry" {
			t.Errorf("expected path './local-registry', got %q", reg.URL)
		}
	})

	t.Run("errors on missing path", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["file"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, nil)
		if err == nil {
			t.Fatal("expected error for missing path")
		}
		if !strings.Contains(err.Error(), "path") {
			t.Errorf("error should mention 'path': %v", err)
		}
	})

	t.Run("String representation", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["file"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("/var/cache/bz"),
		}, nil)

		reg := result.(*RegistryValue)
		str := reg.String()
		if !strings.Contains(str, "file") {
			t.Errorf("String() should contain 'file': %q", str)
		}
		if !strings.Contains(str, "/var/cache/bz") {
			t.Errorf("String() should contain path: %q", str)
		}
	})
}

func TestRegistryGit(t *testing.T) {
	t.Run("creates git registry with URL only", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["git"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("git@github.com:myorg/bcr.git")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reg, ok := result.(*RegistryValue)
		if !ok {
			t.Fatalf("expected *RegistryValue, got %T", result)
		}

		if reg.Kind != RegistryTypeGit {
			t.Errorf("expected type %q, got %q", RegistryTypeGit, reg.Kind)
		}
		if reg.URL != "git@github.com:myorg/bcr.git" {
			t.Errorf("expected URL 'git@github.com:myorg/bcr.git', got %q", reg.URL)
		}
		if reg.Branch != "" {
			t.Errorf("expected empty branch, got %q", reg.Branch)
		}
		if reg.Ref != "" {
			t.Errorf("expected empty ref, got %q", reg.Ref)
		}
	})

	t.Run("creates git registry with branch", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["git"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("git@github.com:myorg/bcr.git")},
			{starlark.String("branch"), starlark.String("main")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reg := result.(*RegistryValue)
		if reg.Branch != "main" {
			t.Errorf("expected branch 'main', got %q", reg.Branch)
		}
	})

	t.Run("creates git registry with ref", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["git"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("git@github.com:myorg/bcr.git")},
			{starlark.String("ref"), starlark.String("v1.0.0")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reg := result.(*RegistryValue)
		if reg.Ref != "v1.0.0" {
			t.Errorf("expected ref 'v1.0.0', got %q", reg.Ref)
		}
	})

	t.Run("exposes branch attribute", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["git"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("git@github.com:myorg/bcr.git")},
			{starlark.String("branch"), starlark.String("develop")},
		})

		reg := result.(*RegistryValue)
		branchAttr, err := reg.Attr("branch")
		if err != nil {
			t.Fatalf("failed to get branch attr: %v", err)
		}
		if string(branchAttr.(starlark.String)) != "develop" {
			t.Errorf("branch attr mismatch")
		}
	})

	t.Run("exposes ref attribute", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["git"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("git@github.com:myorg/bcr.git")},
			{starlark.String("ref"), starlark.String("abc123")},
		})

		reg := result.(*RegistryValue)
		refAttr, err := reg.Attr("ref")
		if err != nil {
			t.Fatalf("failed to get ref attr: %v", err)
		}
		if string(refAttr.(starlark.String)) != "abc123" {
			t.Errorf("ref attr mismatch")
		}
	})

	t.Run("errors when both branch and ref provided", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["git"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("git@github.com:myorg/bcr.git")},
			{starlark.String("branch"), starlark.String("main")},
			{starlark.String("ref"), starlark.String("v1.0.0")},
		})
		if err == nil {
			t.Fatal("expected error when both branch and ref provided")
		}
		if !strings.Contains(err.Error(), "both") {
			t.Errorf("error should mention 'both': %v", err)
		}
	})

	t.Run("errors on missing URL", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["git"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("branch"), starlark.String("main")},
		})
		if err == nil {
			t.Fatal("expected error for missing URL")
		}
	})

	t.Run("String representation with branch", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["git"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("git@github.com:myorg/bcr.git")},
			{starlark.String("branch"), starlark.String("main")},
		})

		reg := result.(*RegistryValue)
		str := reg.String()
		if !strings.Contains(str, "git") {
			t.Errorf("String() should contain 'git': %q", str)
		}
		if !strings.Contains(str, "main") {
			t.Errorf("String() should contain branch: %q", str)
		}
	})

	t.Run("String representation with ref", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["git"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("git@github.com:myorg/bcr.git")},
			{starlark.String("ref"), starlark.String("v2.0.0")},
		})

		reg := result.(*RegistryValue)
		str := reg.String()
		if !strings.Contains(str, "v2.0.0") {
			t.Errorf("String() should contain ref: %q", str)
		}
	})
}

func TestRegistryHTTPPut(t *testing.T) {
	t.Run("creates http_put registry with URL only", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http_put"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("https://registry.internal/bcr")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reg, ok := result.(*RegistryValue)
		if !ok {
			t.Fatalf("expected *RegistryValue, got %T", result)
		}

		if reg.Kind != RegistryTypeHTTPPut {
			t.Errorf("expected type %q, got %q", RegistryTypeHTTPPut, reg.Kind)
		}
		if reg.URL != "https://registry.internal/bcr" {
			t.Errorf("expected URL 'https://registry.internal/bcr', got %q", reg.URL)
		}
		if reg.Auth != nil {
			t.Errorf("expected nil auth, got %v", reg.Auth)
		}
	})

	t.Run("creates http_put registry with auth", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}

		// First create an auth value
		authFn := AuthModule()["bearer_token"].(*starlark.Builtin)
		authResult, err := starlark.Call(thread, authFn, nil, []starlark.Tuple{
			{starlark.String("env"), starlark.String("REGISTRY_TOKEN")},
		})
		if err != nil {
			t.Fatalf("failed to create auth: %v", err)
		}

		// Now create the registry with auth
		fn := RegistryModule()["http_put"].(*starlark.Builtin)
		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("https://registry.internal/bcr")},
			{starlark.String("auth"), authResult},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		reg := result.(*RegistryValue)
		if reg.Auth == nil {
			t.Fatal("expected auth to be set")
		}
		if reg.Auth.EnvVar != "REGISTRY_TOKEN" {
			t.Errorf("expected auth env 'REGISTRY_TOKEN', got %q", reg.Auth.EnvVar)
		}
	})

	t.Run("exposes auth attribute", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}

		authFn := AuthModule()["bearer_token"].(*starlark.Builtin)
		authResult, _ := starlark.Call(thread, authFn, nil, []starlark.Tuple{
			{starlark.String("env"), starlark.String("TOKEN")},
		})

		fn := RegistryModule()["http_put"].(*starlark.Builtin)
		result, _ := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("https://example.com")},
			{starlark.String("auth"), authResult},
		})

		reg := result.(*RegistryValue)
		authAttr, err := reg.Attr("auth")
		if err != nil {
			t.Fatalf("failed to get auth attr: %v", err)
		}
		if authAttr.(*AuthValue).EnvVar != "TOKEN" {
			t.Errorf("auth attr mismatch")
		}
	})

	t.Run("errors on invalid auth type", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http_put"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("https://registry.internal/bcr")},
			{starlark.String("auth"), starlark.String("not-an-auth-value")},
		})
		if err == nil {
			t.Fatal("expected error for invalid auth type")
		}
		if !strings.Contains(err.Error(), "auth") {
			t.Errorf("error should mention 'auth': %v", err)
		}
	})

	t.Run("errors on missing URL", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http_put"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, nil)
		if err == nil {
			t.Fatal("expected error for missing URL")
		}
	})

	t.Run("String representation with auth", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}

		authFn := AuthModule()["bearer_token"].(*starlark.Builtin)
		authResult, _ := starlark.Call(thread, authFn, nil, []starlark.Tuple{
			{starlark.String("env"), starlark.String("TOKEN")},
		})

		fn := RegistryModule()["http_put"].(*starlark.Builtin)
		result, _ := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("url"), starlark.String("https://example.com")},
			{starlark.String("auth"), authResult},
		})

		reg := result.(*RegistryValue)
		str := reg.String()
		if !strings.Contains(str, "http_put") {
			t.Errorf("String() should contain 'http_put': %q", str)
		}
		if !strings.Contains(str, "auth") {
			t.Errorf("String() should contain 'auth': %q", str)
		}
	})
}

func TestRegistryModule(t *testing.T) {
	t.Run("contains all registry functions", func(t *testing.T) {
		module := RegistryModule()

		expectedFuncs := []string{"http", "file", "git", "http_put"}
		for _, name := range expectedFuncs {
			if _, ok := module[name]; !ok {
				t.Errorf("missing function %q in registry module", name)
			}
		}
	})
}

func TestRegistryValueInterfaces(t *testing.T) {
	t.Run("implements starlark.Value", func(t *testing.T) {
		var _ starlark.Value = (*RegistryValue)(nil)
	})

	t.Run("implements starlark.HasAttrs", func(t *testing.T) {
		var _ starlark.HasAttrs = (*RegistryValue)(nil)
	})

	t.Run("AttrNames returns available attributes", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("https://bcr.bazel.build"),
		}, nil)

		reg := result.(*RegistryValue)
		names := reg.AttrNames()

		// Should have at least type and url
		hasType := false
		hasURL := false
		for _, name := range names {
			if name == "type" {
				hasType = true
			}
			if name == "url" {
				hasURL = true
			}
		}
		if !hasType {
			t.Error("AttrNames should include 'type'")
		}
		if !hasURL {
			t.Error("AttrNames should include 'url'")
		}
	})

	t.Run("Truth returns true", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("https://bcr.bazel.build"),
		}, nil)

		reg := result.(*RegistryValue)
		if reg.Truth() != starlark.True {
			t.Error("Truth() should return true")
		}
	})

	t.Run("Attr error for unknown attribute", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		fn := RegistryModule()["http"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, starlark.Tuple{
			starlark.String("https://bcr.bazel.build"),
		}, nil)

		reg := result.(*RegistryValue)
		_, err := reg.Attr("nonexistent")
		if err == nil {
			t.Error("Attr() should return error for unknown attribute")
		}
	})
}
