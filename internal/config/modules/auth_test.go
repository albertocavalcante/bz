package modules

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

func TestAuthBasic(t *testing.T) {
	t.Parallel()
	t.Run("creates basic auth with username and password", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["basic"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("username"), starlark.String("admin")},
			{starlark.String("password"), starlark.String("secret")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth, ok := result.(*AuthValue)
		if !ok {
			t.Fatalf("expected *AuthValue, got %T", result)
		}

		if auth.Kind != AuthTypeBasic {
			t.Errorf("expected type %q, got %q", AuthTypeBasic, auth.Kind)
		}
		if auth.Username != "admin" {
			t.Errorf("expected username 'admin', got %q", auth.Username)
		}
		if auth.Password != "secret" {
			t.Errorf("expected password 'secret', got %q", auth.Password)
		}
	})

	t.Run("exposes type attribute", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["basic"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("username"), starlark.String("user")},
			{starlark.String("password"), starlark.String("pass")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth := result.(*AuthValue)
		typeAttr, err := auth.Attr("type")
		if err != nil {
			t.Fatalf("failed to get type attr: %v", err)
		}
		if string(typeAttr.(starlark.String)) != AuthTypeBasic {
			t.Errorf("type attr mismatch")
		}
	})

	t.Run("exposes username attribute", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["basic"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("username"), starlark.String("testuser")},
			{starlark.String("password"), starlark.String("pass")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth := result.(*AuthValue)
		usernameAttr, err := auth.Attr("username")
		if err != nil {
			t.Fatalf("failed to get username attr: %v", err)
		}
		if string(usernameAttr.(starlark.String)) != "testuser" {
			t.Errorf("username attr mismatch")
		}
	})

	t.Run("errors on missing username", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["basic"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("password"), starlark.String("secret")},
		})
		if err == nil {
			t.Fatal("expected error for missing username")
		}
		if !strings.Contains(err.Error(), "username") {
			t.Errorf("error should mention 'username': %v", err)
		}
	})

	t.Run("errors on missing password", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["basic"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("username"), starlark.String("admin")},
		})
		if err == nil {
			t.Fatal("expected error for missing password")
		}
		if !strings.Contains(err.Error(), "password") {
			t.Errorf("error should mention 'password': %v", err)
		}
	})

	t.Run("String representation", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["basic"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("username"), starlark.String("admin")},
			{starlark.String("password"), starlark.String("secret")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth := result.(*AuthValue)
		str := auth.String()
		if !strings.Contains(str, "basic") {
			t.Errorf("String() should contain 'basic': %q", str)
		}
		if !strings.Contains(str, "admin") {
			t.Errorf("String() should contain username: %q", str)
		}
	})
}

func TestAuthBearerToken(t *testing.T) {
	t.Parallel()
	t.Run("creates bearer token with env var", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["bearer_token"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("env"), starlark.String("REGISTRY_TOKEN")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth, ok := result.(*AuthValue)
		if !ok {
			t.Fatalf("expected *AuthValue, got %T", result)
		}

		if auth.Kind != AuthTypeBearerToken {
			t.Errorf("expected type %q, got %q", AuthTypeBearerToken, auth.Kind)
		}
		if auth.EnvVar != "REGISTRY_TOKEN" {
			t.Errorf("expected env 'REGISTRY_TOKEN', got %q", auth.EnvVar)
		}
		if auth.TokenValue != "" {
			t.Errorf("expected empty token value, got %q", auth.TokenValue)
		}
	})

	t.Run("creates bearer token with static value", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["bearer_token"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("value"), starlark.String("my-secret-token")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth := result.(*AuthValue)
		if auth.TokenValue != "my-secret-token" {
			t.Errorf("expected token value 'my-secret-token', got %q", auth.TokenValue)
		}
		if auth.EnvVar != "" {
			t.Errorf("expected empty env var, got %q", auth.EnvVar)
		}
	})

	t.Run("exposes env attribute", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["bearer_token"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("env"), starlark.String("MY_TOKEN")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth := result.(*AuthValue)
		envAttr, err := auth.Attr("env")
		if err != nil {
			t.Fatalf("failed to get env attr: %v", err)
		}
		if string(envAttr.(starlark.String)) != "MY_TOKEN" {
			t.Errorf("env attr mismatch")
		}
	})

	t.Run("errors when neither env nor value provided", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["bearer_token"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, nil)
		if err == nil {
			t.Fatal("expected error when neither env nor value provided")
		}
		if !strings.Contains(err.Error(), "env") || !strings.Contains(err.Error(), "value") {
			t.Errorf("error should mention 'env' and 'value': %v", err)
		}
	})

	t.Run("errors when both env and value provided", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["bearer_token"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("env"), starlark.String("TOKEN")},
			{starlark.String("value"), starlark.String("secret")},
		})
		if err == nil {
			t.Fatal("expected error when both env and value provided")
		}
		if !strings.Contains(err.Error(), "both") {
			t.Errorf("error should mention 'both': %v", err)
		}
	})

	t.Run("String representation with env", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["bearer_token"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("env"), starlark.String("MY_TOKEN")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth := result.(*AuthValue)
		str := auth.String()
		if !strings.Contains(str, "bearer_token") {
			t.Errorf("String() should contain 'bearer_token': %q", str)
		}
		if !strings.Contains(str, "MY_TOKEN") {
			t.Errorf("String() should contain env var: %q", str)
		}
	})

	t.Run("String representation with value hides token", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["bearer_token"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("value"), starlark.String("super-secret")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth := result.(*AuthValue)
		str := auth.String()
		if strings.Contains(str, "super-secret") {
			t.Errorf("String() should NOT contain the secret token: %q", str)
		}
		if !strings.Contains(str, "***") {
			t.Errorf("String() should contain masked token: %q", str)
		}
	})
}

func TestAuthHeader(t *testing.T) {
	t.Parallel()
	t.Run("creates header auth", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["header"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("name"), starlark.String("X-Api-Key")},
			{starlark.String("value"), starlark.String("my-api-key")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth, ok := result.(*AuthValue)
		if !ok {
			t.Fatalf("expected *AuthValue, got %T", result)
		}

		if auth.Kind != AuthTypeHeader {
			t.Errorf("expected type %q, got %q", AuthTypeHeader, auth.Kind)
		}
		if auth.HeaderName != "X-Api-Key" {
			t.Errorf("expected header name 'X-Api-Key', got %q", auth.HeaderName)
		}
		if auth.HeaderValue != "my-api-key" {
			t.Errorf("expected header value 'my-api-key', got %q", auth.HeaderValue)
		}
	})

	t.Run("exposes name attribute", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["header"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("name"), starlark.String("Authorization")},
			{starlark.String("value"), starlark.String("token")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth := result.(*AuthValue)
		nameAttr, err := auth.Attr("name")
		if err != nil {
			t.Fatalf("failed to get name attr: %v", err)
		}
		if string(nameAttr.(starlark.String)) != "Authorization" {
			t.Errorf("name attr mismatch")
		}
	})

	t.Run("errors on missing name", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["header"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("value"), starlark.String("token")},
		})
		if err == nil {
			t.Fatal("expected error for missing name")
		}
	})

	t.Run("errors on missing value", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["header"].(*starlark.Builtin)

		_, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("name"), starlark.String("X-Api-Key")},
		})
		if err == nil {
			t.Fatal("expected error for missing value")
		}
	})

	t.Run("String representation", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["header"].(*starlark.Builtin)

		result, err := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("name"), starlark.String("X-Custom")},
			{starlark.String("value"), starlark.String("secret")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		auth := result.(*AuthValue)
		str := auth.String()
		if !strings.Contains(str, "header") {
			t.Errorf("String() should contain 'header': %q", str)
		}
		if !strings.Contains(str, "X-Custom") {
			t.Errorf("String() should contain header name: %q", str)
		}
	})
}

func TestAuthModule(t *testing.T) {
	t.Parallel()
	t.Run("contains all auth functions", func(t *testing.T) {
		t.Parallel()
		module := AuthModule()

		expectedFuncs := []string{"basic", "bearer_token", "header"}
		for _, name := range expectedFuncs {
			if _, ok := module[name]; !ok {
				t.Errorf("missing function %q in auth module", name)
			}
		}
	})
}

func TestAuthValueInterfaces(t *testing.T) {
	t.Parallel()
	t.Run("implements starlark.Value", func(t *testing.T) {
		t.Parallel()
		var _ starlark.Value = (*AuthValue)(nil)
	})

	t.Run("implements starlark.HasAttrs", func(t *testing.T) {
		t.Parallel()
		var _ starlark.HasAttrs = (*AuthValue)(nil)
	})

	t.Run("AttrNames returns available attributes", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["basic"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("username"), starlark.String("user")},
			{starlark.String("password"), starlark.String("pass")},
		})

		auth := result.(*AuthValue)
		names := auth.AttrNames()

		// Should have at least type and username
		hasType := false
		hasUsername := false
		for _, name := range names {
			if name == "type" {
				hasType = true
			}
			if name == "username" {
				hasUsername = true
			}
		}
		if !hasType {
			t.Error("AttrNames should include 'type'")
		}
		if !hasUsername {
			t.Error("AttrNames should include 'username'")
		}
	})

	t.Run("Truth returns true", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["basic"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("username"), starlark.String("user")},
			{starlark.String("password"), starlark.String("pass")},
		})

		auth := result.(*AuthValue)
		if auth.Truth() != starlark.True {
			t.Error("Truth() should return true")
		}
	})

	t.Run("Type returns auth", func(t *testing.T) {
		t.Parallel()
		thread := &starlark.Thread{Name: "test"}
		fn := AuthModule()["basic"].(*starlark.Builtin)

		result, _ := starlark.Call(thread, fn, nil, []starlark.Tuple{
			{starlark.String("username"), starlark.String("user")},
			{starlark.String("password"), starlark.String("pass")},
		})

		auth := result.(*AuthValue)
		if auth.Kind != "basic" {
			t.Errorf("Type should be 'basic', got %q", auth.Kind)
		}
	})
}
