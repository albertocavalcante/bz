//go:build unix

package bzconfig

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenNoFollow pins the real open flags: no Load-level test uses the
// OS environment's Open, so this is the test that fails if O_NOFOLLOW or
// O_NONBLOCK is dropped.
func TestOpenNoFollow(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.toml")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o600))

	t.Run("a symlink is not followed", func(t *testing.T) {
		link := filepath.Join(dir, "link.toml")
		require.NoError(t, os.Symlink(target, link))
		f, err := openNoFollow(link)
		if f != nil {
			_ = f.Close()
		}
		require.Error(t, err)
		assert.True(t, errors.Is(err, syscall.ELOOP), "err = %v, want ELOOP", err)
	})

	t.Run("a FIFO opens without blocking and is refused", func(t *testing.T) {
		fifo := filepath.Join(dir, "fifo.toml")
		require.NoError(t, syscall.Mkfifo(fifo, 0o600))
		resolved, err := filepath.EvalSymlinks(fifo)
		require.NoError(t, err)
		where := fifo
		if resolved != fifo {
			where = fifo + " (resolved to " + resolved + ")"
		}
		_, _, err = OSEnvironment().readConfigFile(LayerProject, fifo, nil)
		require.EqualError(t, err, "untrusted project config "+where+": not a regular file; fix: replace it with a regular file, "+
			"or list "+filepath.Dir(resolved)+" in BZ_TRUSTED_CONFIG_DIRS if the shared setup is intended")
	})
}
