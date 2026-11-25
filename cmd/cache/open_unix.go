//go:build unix

package cache

import (
	"os"
	"syscall"
)

func verificationOpenFlags() int {
	return os.O_RDONLY | syscall.O_NONBLOCK
}
