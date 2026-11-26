package testutil

import (
	"github.com/albertocavalcante/bz/internal/bzconfig"
	"github.com/albertocavalcante/bz/internal/cli"
)

// IsolatedConfig is a testenv.Run setup: bz reads no system file and only a
// user file under home.
func IsolatedConfig(home string) (undo func()) {
	return cli.SetEnvironment(bzconfig.OSEnvironment().Isolated(home))
}
