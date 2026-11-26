package cli

import (
	"os"
	"testing"

	"github.com/albertocavalcante/bz/internal/bzconfig"
	"github.com/albertocavalcante/bz/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Run(m, func(home string) func() {
		return SetEnvironment(bzconfig.OSEnvironment().Isolated(home))
	}))
}
