package mod

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	Configure()
	os.Exit(m.Run())
}
