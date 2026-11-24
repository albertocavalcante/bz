package testenv

import (
	"os"
	"testing"
)

func TestIsolate(t *testing.T) {
	t.Setenv("BZ_OFFLINE", "1")
	t.Setenv("BZ_REGISTRY", "https://example.invalid")
	home := os.Getenv("HOME")

	dir, restore, err := Isolate()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"BZ_OFFLINE", "BZ_REGISTRY"} {
		if v, ok := os.LookupEnv(name); ok {
			t.Errorf("%s = %q, want unset", name, v)
		}
	}
	if got := os.Getenv("HOME"); got != dir {
		t.Errorf("HOME = %q, want %q", got, dir)
	}

	restore()
	if got := os.Getenv("BZ_OFFLINE"); got != "1" {
		t.Errorf("BZ_OFFLINE after restore = %q, want 1", got)
	}
	if got := os.Getenv("HOME"); got != home {
		t.Errorf("HOME after restore = %q, want %q", got, home)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("%s still exists", dir)
	}
}
