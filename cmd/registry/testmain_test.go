package registry

import (
	"os"
	"testing"

	"github.com/albertocavalcante/bz/internal/testenv"
)

func TestMain(m *testing.M) {
	Configure()
	os.Exit(testenv.Run(m))
}
