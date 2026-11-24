//go:build !windows

package bzconfig

import (
	"io/fs"
	"syscall"
)

// fileOwner returns the owner uid recorded in fi.
func fileOwner(fi fs.FileInfo) (int, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(st.Uid), true
}
