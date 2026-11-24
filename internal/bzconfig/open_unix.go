//go:build unix

package bzconfig

import (
	"io/fs"
	"os"
	"syscall"
)

// openNoFollow opens name for reading without following a final symbolic
// link and without blocking if it is a FIFO.
func openNoFollow(name string) (fs.File, error) {
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
