package testenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsolate(t *testing.T) {
	type before struct {
		set   bool
		value string
	}
	tests := []struct {
		name   string
		before before
	}{
		{name: "unset stays unset", before: before{}},
		{name: "empty stays empty", before: before{set: true}},
		{name: "value is restored", before: before{set: true, value: "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BZ_OFFLINE", "") // registers restoration of the real value
			if tt.before.set {
				os.Setenv("BZ_OFFLINE", tt.before.value)
			} else {
				os.Unsetenv("BZ_OFFLINE")
			}

			dir, restore, err := Isolate()
			if err != nil {
				t.Fatal(err)
			}
			if v, ok := os.LookupEnv("BZ_OFFLINE"); ok {
				t.Errorf("BZ_OFFLINE = %q during isolation, want unset", v)
			}
			restore()

			v, ok := os.LookupEnv("BZ_OFFLINE")
			if ok != tt.before.set || v != tt.before.value {
				t.Errorf("after restore: BZ_OFFLINE = %q (set %v), want %q (set %v)", v, ok, tt.before.value, tt.before.set)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Errorf("%s still exists", dir)
			}
		})
	}
}

func TestIsolate_RedirectsEveryDirectoryVariable(t *testing.T) {
	want := map[string]string{
		"HOME": "", "USERPROFILE": "",
		"XDG_CONFIG_HOME": "config", "XDG_CACHE_HOME": "cache",
		"AppData": "appdata", "LocalAppData": "localappdata",
	}
	old := map[string]string{}
	for name := range want {
		old[name] = os.Getenv(name)
	}

	dir, restore, err := Isolate()
	if err != nil {
		t.Fatal(err)
	}
	for name, sub := range want {
		if got := os.Getenv(name); got != filepath.Join(dir, sub) {
			t.Errorf("%s = %q, want %q", name, got, filepath.Join(dir, sub))
		}
	}
	restore()
	for name, v := range old {
		if got := os.Getenv(name); got != v {
			t.Errorf("%s after restore = %q, want %q", name, got, v)
		}
	}
}

func TestRun_AppliesSetupsWithTheHome(t *testing.T) {
	var got string
	undone := false
	setup := func(home string) func() {
		got = home
		if os.Getenv("HOME") != home {
			t.Errorf("setup ran before isolation: HOME = %q, home = %q", os.Getenv("HOME"), home)
		}
		return func() { undone = true }
	}
	if code := Run(fakeM{}, setup); code != 0 {
		t.Fatalf("Run = %d", code)
	}
	if got == "" || !undone {
		t.Errorf("setup home %q, undone %v", got, undone)
	}
}

type fakeM struct{}

func (fakeM) Run() int { return 0 }
