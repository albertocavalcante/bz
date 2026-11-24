//go:build !unix

package bzconfig

import "io/fs"

// fileOwner reports no owner on platforms without Unix file ownership.
// Windows skips the ownership rules; elsewhere an unknown owner is refused.
func fileOwner(fs.FileInfo) (int, bool) { return 0, false }
