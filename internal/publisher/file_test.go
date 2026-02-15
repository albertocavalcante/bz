package publisher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFilePublisher_Type(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pub, err := NewFilePublisher(dir)
	if err != nil {
		t.Fatalf("NewFilePublisher() error = %v", err)
	}

	if got := pub.Type(); got != TypeFile {
		t.Errorf("Type() = %q, want %q", got, TypeFile)
	}
}

func TestFilePublisher_Put(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pub, err := NewFilePublisher(dir)
	if err != nil {
		t.Fatalf("NewFilePublisher() error = %v", err)
	}

	ctx := context.Background()
	content := []byte(`module(name = "rules_go", version = "0.50.0")`)

	// Put a file with nested directories
	err = pub.Put(ctx, "rules_go/0.50.0/MODULE.bazel", content)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(dir, "rules_go/0.50.0/MODULE.bazel")
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("file content = %q, want %q", string(data), string(content))
	}
}

func TestFilePublisher_Exists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pub, err := NewFilePublisher(dir)
	if err != nil {
		t.Fatalf("NewFilePublisher() error = %v", err)
	}

	ctx := context.Background()

	// File doesn't exist yet
	exists, err := pub.Exists(ctx, "rules_go/0.50.0/MODULE.bazel")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}

	// Create the file
	content := []byte(`module(name = "rules_go", version = "0.50.0")`)
	if err := pub.Put(ctx, "rules_go/0.50.0/MODULE.bazel", content); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Now it should exist
	exists, err = pub.Exists(ctx, "rules_go/0.50.0/MODULE.bazel")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}
}

func TestFilePublisher_Finalize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pub, err := NewFilePublisher(dir)
	if err != nil {
		t.Fatalf("NewFilePublisher() error = %v", err)
	}

	ctx := context.Background()

	// Finalize should be a no-op
	err = pub.Finalize(ctx, "test commit")
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
}

func TestFilePublisher_Close(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pub, err := NewFilePublisher(dir)
	if err != nil {
		t.Fatalf("NewFilePublisher() error = %v", err)
	}

	// Close should be a no-op
	err = pub.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFilePublisher_CreatesBaseDir(t *testing.T) {
	t.Parallel(
	// Create a path that doesn't exist yet
	)

	baseDir := filepath.Join(t.TempDir(), "nested", "registry")

	pub, err := NewFilePublisher(baseDir)
	if err != nil {
		t.Fatalf("NewFilePublisher() error = %v", err)
	}

	// Verify the directory was created
	info, err := os.Stat(baseDir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Error("basePath is not a directory")
	}

	// Clean up
	pub.Close()
}

func TestFilePublisher_PutOverwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pub, err := NewFilePublisher(dir)
	if err != nil {
		t.Fatalf("NewFilePublisher() error = %v", err)
	}

	ctx := context.Background()
	path := "rules_go/0.50.0/MODULE.bazel"

	// Write initial content
	content1 := []byte(`module(name = "rules_go", version = "0.50.0")`)
	if err := pub.Put(ctx, path, content1); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Overwrite with new content
	content2 := []byte(`module(name = "rules_go", version = "0.50.1")`)
	if err := pub.Put(ctx, path, content2); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Verify new content
	fullPath := filepath.Join(dir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != string(content2) {
		t.Errorf("file content = %q, want %q", string(data), string(content2))
	}
}
