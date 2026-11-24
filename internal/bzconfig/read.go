package bzconfig

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// maxConfigSize bounds how much of a config file is read.
const maxConfigSize = 1 << 20

// readConfigFile opens a config file once and returns its contents read
// from that same handle, after applying the rules for its layer to the
// handle's own FileInfo (fstat), so the file that was checked is the file
// that is read. A symbolic link is resolved first and the target is opened
// without following links, so the checks apply to what was opened. It
// reports ok=false when the file does not exist.
//
// Rules outside Windows (Windows only requires a regular file):
//   - project files: [CheckProjectConfigTrust] (owner, mode, directory);
//   - user files: owned by the current user or root, not writable by
//     group or others;
//   - system files: owned by root, not writable by group or others.
//
// A file in a directory listed in BZ_TRUSTED_CONFIG_DIRS skips the owner
// and mode rules for every layer.
func (e Environment) readConfigFile(kind LayerKind, path string, trusted []fs.FileInfo) (data []byte, ok bool, err error) {
	var missing []string
	if e.Open == nil {
		missing = append(missing, "Open")
	}
	if e.EvalSymlinks == nil {
		missing = append(missing, "EvalSymlinks")
	}
	if len(missing) > 0 {
		return nil, false, fmt.Errorf("cannot read configuration: the environment does not provide %s", strings.Join(missing, ", "))
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
		err = e.checkTrustInfo(path, target, fi, trusted)
	} else {
		err = e.checkOwnFile(kind, path, target, fi, trusted)
	}
	if err != nil {
		return nil, false, err
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

// checkOwnFile applies the lighter system/user rules: the file itself must
// be regular, owned by root (system) or the current user or root (user),
// and not writable by group or others. Its directory is not checked: the
// user file lives in the user's own config directory and the system file
// under a root-owned /etc.
func (e Environment) checkOwnFile(kind LayerKind, path, target string, fi fs.FileInfo, trusted []fs.FileInfo) error {
	dir := filepath.Dir(target)
	refuse := func(reason UntrustedReason, detail, fix string) error {
		return &UntrustedConfigError{Path: path, Target: target, Dir: dir, Layer: kind, Reason: reason, Detail: detail, Fix: fix}
	}
	if !fi.Mode().IsRegular() {
		return refuse(ReasonNotRegular, "not a regular file", "replace it with a regular file")
	}
	if e.GOOS == goosWindows {
		return nil
	}
	if inTrusted, err := e.isAnyOf(dir, trusted); err != nil || inTrusted {
		return err
	}
	if e.Getuid == nil || e.FileOwner == nil {
		return refuse(ReasonOwnerUnknown, "its owner cannot be determined", "check its ownership")
	}
	uid := e.Getuid()
	owner, known := e.FileOwner(fi)
	if !known {
		return refuse(ReasonOwnerUnknown, "its owner cannot be determined", "check its ownership")
	}
	switch {
	case kind == LayerSystem && owner != 0:
		return refuse(ReasonFileOwner, fmt.Sprintf("owned by uid %d, not root", owner),
			"have an administrator make root its owner")
	case kind != LayerSystem && owner != uid && owner != 0:
		return refuse(ReasonFileOwner, fmt.Sprintf("owned by uid %d, not you (uid %d) or root", owner, uid),
			fmt.Sprintf("recreate it as yourself, or have an administrator change its owner (it belongs to uid %d)", owner))
	}
	if fi.Mode().Perm()&0o022 != 0 {
		fix := "have an administrator run chmod go-w on it"
		switch {
		case !printable(target):
			fix = "rename the file to remove control or non-printable characters from its path, then fix its permissions"
		case owner == uid:
			fix = "chmod go-w -- " + shellQuote(target)
		}
		return refuse(ReasonFileWritable, fmt.Sprintf("writable by group or others (mode %s)", fi.Mode().Perm()), fix)
	}
	return nil
}
