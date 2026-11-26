//go:build unix

package registry

import (
	"os"
	"syscall"
)

func registryReadFlags() int {
	return os.O_RDONLY | syscall.O_NONBLOCK
}

// Force directory traversal before opening the last component. A direct
// directory open can otherwise block on a FIFO before checking its type.
func registryDirectoryPath(path string) string {
	return path + "/."
}
