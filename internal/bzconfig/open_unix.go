//go:build unix

package bzconfig

import (
	"io/fs"
	"os"
	"syscall"
)

// openNoFollow opens name for reading. Per open(2), O_NOFOLLOW makes the open
// fail with ELOOP when the final component is a symbolic link, and O_NONBLOCK
// stops an open of a FIFO for reading from waiting for a writer.
func openNoFollow(name string) (fs.File, error) {
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
