//go:build unix

package publisher

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryReadFIFOIsNonblocking(t *testing.T) {
	dest := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dest, "modules", "example"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(filepath.Join(dest, "modules", "example", "file"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := NewFileRegistryPublisher(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	done := make(chan error, 1)
	go func() { _, err := p.Read("example/file"); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("read FIFO succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("read FIFO blocked")
	}
}
