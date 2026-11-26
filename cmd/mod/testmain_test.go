package mod

import (
	"os"
	"testing"

	"github.com/albertocavalcante/bz/internal/testenv"
	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestMain(m *testing.M) {
	Configure()
	os.Exit(testenv.Run(m, testutil.IsolatedConfig))
}
