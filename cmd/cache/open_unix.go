//go:build unix

package cache

import (
	"os"
	"syscall"
)

func verificationOpenFlags() int {
	return os.O_RDONLY | syscall.O_NONBLOCK
}

// Force directory traversal before opening the last component. A direct
// directory open can otherwise block on a FIFO before checking its type.
func verificationDirectoryPath(path string) string {
	return path + "/."
}
