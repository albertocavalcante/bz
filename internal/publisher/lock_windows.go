//go:build windows

package publisher

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
)

func lockFlags() int { return os.O_RDWR }
func tryLock(f *os.File) error {
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
}
func unlock(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &windows.Overlapped{})
}
func lockBusy(err error) bool { return errors.Is(err, windows.ERROR_LOCK_VIOLATION) }
