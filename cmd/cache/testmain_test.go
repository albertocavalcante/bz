package cache

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	Configure()
	os.Exit(m.Run())
}
