package publisher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryLockCancellation(t *testing.T) {
	dest := t.TempDir()
	first, err := NewFileRegistryPublisher(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewFileRegistryPublisher(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	release, err := first.LockRegistry(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = second.LockRegistry(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock error = %v", err)
	}
	release()
	release, err = second.LockRegistry(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestRegistryReadBounded(t *testing.T) {
	dest := t.TempDir()
	p, err := NewFileRegistryPublisher(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Put(context.Background(), "example/file", []byte("ok")); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dest, "modules", "example", "file"), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(51 << 20); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Read("example/file"); err == nil {
		t.Fatal("oversized read succeeded")
	}
}
