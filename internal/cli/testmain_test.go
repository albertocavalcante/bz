package cli

import (
	"fmt"
	"os"
	"testing"

	"github.com/albertocavalcante/bz/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(runIsolated(m))
}

func runIsolated(m *testing.M) int {
	_, restore, err := testenv.Isolate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer restore()
	return m.Run()
}
