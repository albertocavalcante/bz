//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package publisher

import (
	"errors"
	"os"
)

func lockFlags() int         { return os.O_CREATE | os.O_RDWR }
func tryLock(*os.File) error { return errors.New("file registry locking unsupported on this platform") }
func unlock(*os.File) error  { return nil }
func lockBusy(error) bool    { return false }
