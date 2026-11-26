package publisher

import (
	"context"
	"fmt"
	"time"
)

// LockRegistry serializes cooperating writers. The marker remains on disk;
// ownership is held by the OS and released when the handle closes.
func (p *FilePublisher) LockRegistry(ctx context.Context) (func(), error) {
	f, err := p.root.OpenFile(".bz-sync.lock", lockFlags(), 0o600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("lock marker is not a regular file")
	}
	for {
		err = tryLock(f)
		if err == nil {
			return func() { _ = unlock(f); _ = f.Close() }, nil
		}
		if !lockBusy(err) {
			_ = f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
