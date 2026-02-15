package publisher

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPPublisher_Type(t *testing.T) {
	t.Parallel()
	pub, err := NewHTTPPublisher("https://example.com/registry", nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	if got := pub.Type(); got != TypeHTTPPut {
		t.Errorf("Type() = %q, want %q", got, TypeHTTPPut)
	}
}

func TestHTTPPublisher_Put(t *testing.T) {
	t.Parallel()
	var receivedBody []byte
	var receivedPath string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			receivedPath = r.URL.Path
			receivedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			receivedBody = body
			w.WriteHeader(http.StatusCreated)
			return
		}
		// MKCOL for directory creation
		if r.Method == "MKCOL" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	pub, err := NewHTTPPublisher(server.URL, nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()
	content := []byte(`module(name = "rules_go", version = "0.50.0")`)

	err = pub.Put(ctx, "rules_go/0.50.0/MODULE.bazel", content)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if receivedPath != "/rules_go/0.50.0/MODULE.bazel" {
		t.Errorf("received path = %q, want %q", receivedPath, "/rules_go/0.50.0/MODULE.bazel")
	}

	if string(receivedBody) != string(content) {
		t.Errorf("received body = %q, want %q", string(receivedBody), string(content))
	}

	if receivedContentType != "text/plain" {
		t.Errorf("received Content-Type = %q, want %q", receivedContentType, "text/plain")
	}
}

func TestHTTPPublisher_Put_JSONContentType(t *testing.T) {
	t.Parallel()
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			receivedContentType = r.Header.Get("Content-Type")
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.Method == "MKCOL" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	pub, err := NewHTTPPublisher(server.URL, nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()
	content := []byte(`{"integrity": "sha256-abc123"}`)

	err = pub.Put(ctx, "rules_go/0.50.0/source.json", content)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if receivedContentType != "application/json" {
		t.Errorf("received Content-Type = %q, want %q", receivedContentType, "application/json")
	}
}

func TestHTTPPublisher_Put_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pub, err := NewHTTPPublisher(server.URL, nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()
	content := []byte(`test content`)

	err = pub.Put(ctx, "test/file.txt", content)
	if err == nil {
		t.Error("Put() expected error for 403 response")
	}
}

func TestHTTPPublisher_Exists(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		switch r.URL.Path {
		case "/exists.txt":
			w.WriteHeader(http.StatusOK)
		case "/notfound.txt":
			w.WriteHeader(http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	pub, err := NewHTTPPublisher(server.URL, nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()

	// File exists
	exists, err := pub.Exists(ctx, "exists.txt")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}

	// File doesn't exist
	exists, err = pub.Exists(ctx, "notfound.txt")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}
}

func TestHTTPPublisher_BasicAuth(t *testing.T) {
	t.Parallel()
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	auth := &Auth{
		Type:     AuthTypeBasic,
		Username: "user",
		Password: "pass",
	}

	pub, err := NewHTTPPublisher(server.URL, auth)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()
	err = pub.Put(ctx, "test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Basic auth header for "user:pass" is "Basic dXNlcjpwYXNz"
	expected := "Basic dXNlcjpwYXNz"
	if receivedAuth != expected {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, expected)
	}
}

func TestHTTPPublisher_BearerAuth(t *testing.T) {
	t.Parallel()
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	auth := &Auth{
		Type:  AuthTypeBearer,
		Token: "my-secret-token",
	}

	pub, err := NewHTTPPublisher(server.URL, auth)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()
	err = pub.Put(ctx, "test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	expected := "Bearer my-secret-token"
	if receivedAuth != expected {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, expected)
	}
}

func TestHTTPPublisher_BearerAuthFromEnv(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	// Set environment variable
	t.Setenv("TEST_TOKEN", "env-token-value")

	auth := &Auth{
		Type:   AuthTypeBearer,
		EnvVar: "TEST_TOKEN",
	}

	pub, err := NewHTTPPublisher(server.URL, auth)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()
	err = pub.Put(ctx, "test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	expected := "Bearer env-token-value"
	if receivedAuth != expected {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, expected)
	}
}

func TestHTTPPublisher_CustomHeaderAuth(t *testing.T) {
	t.Parallel()
	var receivedHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	auth := &Auth{
		Type:        AuthTypeHeader,
		HeaderName:  "X-Api-Key",
		HeaderValue: "secret-api-key",
	}

	pub, err := NewHTTPPublisher(server.URL, auth)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()
	err = pub.Put(ctx, "test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if receivedHeader != "secret-api-key" {
		t.Errorf("X-Api-Key header = %q, want %q", receivedHeader, "secret-api-key")
	}
}

func TestHTTPPublisher_Finalize(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pub, err := NewHTTPPublisher(server.URL, nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()

	// Finalize should be a no-op
	err = pub.Finalize(ctx, "test commit")
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
}

func TestHTTPPublisher_Close(t *testing.T) {
	t.Parallel()
	pub, err := NewHTTPPublisher("https://example.com", nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	// Close should be a no-op
	err = pub.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestHTTPPublisher_EmptyURL(t *testing.T) {
	t.Parallel()
	_, err := NewHTTPPublisher("", nil)
	if err == nil {
		t.Error("NewHTTPPublisher() expected error for empty URL")
	}
}

func TestHTTPPublisher_URLNormalization(t *testing.T) {
	t.Parallel()
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	// URL with trailing slash should be normalized
	pub, err := NewHTTPPublisher(server.URL+"/", nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}

	ctx := context.Background()
	err = pub.Put(ctx, "test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Should not have double slashes
	if receivedPath != "/test.txt" {
		t.Errorf("received path = %q, want %q", receivedPath, "/test.txt")
	}
}
