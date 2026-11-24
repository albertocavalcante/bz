package bzconfig

import (
	"os"
	"testing"

	"github.com/albertocavalcante/bz/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Run(m))
}
