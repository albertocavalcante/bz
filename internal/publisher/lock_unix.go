//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package publisher

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func lockFlags() int           { return os.O_CREATE | os.O_RDWR | unix.O_NONBLOCK | unix.O_NOFOLLOW }
func tryLock(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
func unlock(f *os.File) error  { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
func lockBusy(err error) bool  { return errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) }
