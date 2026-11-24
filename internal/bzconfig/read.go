package bzconfig

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
)

// maxConfigSize bounds how much of a config file is read.
const maxConfigSize = 1 << 20

// readConfigFile opens a config file once and returns its contents read
// from that same handle. The checks run on the handle's own FileInfo
// (fstat), so the file that was checked is the file that is read. A
// symbolic link is resolved first and the target is opened without
// following links, so the checks apply to what was opened. Project files
// must pass [CheckProjectConfigTrust]; system and user files must be
// regular files. It reports ok=false when the file does not exist.
func (e Environment) readConfigFile(kind LayerKind, path string, trusted []fs.FileInfo) (data []byte, ok bool, err error) {
	if e.Open == nil {
		return nil, false, errors.New("cannot read configuration: the environment does not provide Open")
	}
	target, err := e.resolve(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("reading %s %s: %w", kind, DisplayPath(path), cleanErr(err))
	}
	f, err := e.Open(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("reading %s %s: %w", kind, DisplayPath(path), cleanErr(err))
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("reading %s %s: %w", kind, DisplayPath(path), cleanErr(err))
	}
	if kind == LayerProject {
		if err := e.checkTrustInfo(path, target, fi, trusted); err != nil {
			return nil, false, err
		}
	} else if !fi.Mode().IsRegular() {
		return nil, false, fmt.Errorf("reading %s %s: not a regular file", kind, DisplayPath(path))
	}

	data, err = io.ReadAll(io.LimitReader(f, maxConfigSize+1))
	if err != nil {
		return nil, false, fmt.Errorf("reading %s %s: %w", kind, DisplayPath(path), cleanErr(err))
	}
	if len(data) > maxConfigSize {
		return nil, false, fmt.Errorf("reading %s %s: larger than %d bytes", kind, DisplayPath(path), maxConfigSize)
	}
	return data, true, nil
}
