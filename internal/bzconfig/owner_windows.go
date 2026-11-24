//go:build windows

package bzconfig

import "io/fs"

// fileOwner reports no owner: Windows ownership does not map to the
// Unix-style trust rules, which are skipped there.
func fileOwner(fs.FileInfo) (int, bool) { return 0, false }
