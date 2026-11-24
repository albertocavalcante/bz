//go:build !unix

package bzconfig

import (
	"io/fs"
	"os"
)

// openNoFollow opens name for reading. Platforms without O_NOFOLLOW open
// the resolved path normally.
func openNoFollow(name string) (fs.File, error) {
	return os.Open(name)
}
