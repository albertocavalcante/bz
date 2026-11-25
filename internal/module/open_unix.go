//go:build unix

package module

import (
	"os"
	"syscall"
)

// verificationOpenFlags prevents a verification read from blocking on a FIFO
// and prevents following a directly opened final path. os.Root continues to
// resolve confined symlinks internally.
func verificationOpenFlags() int {
	return os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
}

func openForVerification(path string) (*os.File, error) {
	return os.OpenFile(path, verificationOpenFlags(), 0)
}
