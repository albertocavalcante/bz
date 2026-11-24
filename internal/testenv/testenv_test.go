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

func TestRun(t *testing.T) {
	var events []string
	setup := func(name string) func(string) func() {
		return func(home string) func() {
			if os.Getenv("HOME") != home {
				t.Errorf("setup %s ran before isolation: HOME = %q, home = %q", name, os.Getenv("HOME"), home)
			}
			events = append(events, "setup "+name)
			return func() { events = append(events, "undo "+name) }
		}
	}

	if code := Run(fakeM{code: 3}, setup("a"), setup("b")); code != 3 {
		t.Errorf("Run = %d, want the tests' exit code 3", code)
	}
	want := []string{"setup a", "setup b", "undo b", "undo a"}
	if len(events) != len(want) {
		t.Fatalf("events = %q, want %q", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %q, want %q", events, want)
		}
	}
}

func TestRun_PanickingSetupUndoesEarlierSetups(t *testing.T) {
	undone := false
	defer func() {
		if recover() == nil {
			t.Error("Run did not propagate the panic")
		}
		if !undone {
			t.Error("the first setup was not undone")
		}
	}()
	Run(fakeM{},
		func(string) func() { return func() { undone = true } },
		func(string) func() { panic("setup failed") },
	)
}

type fakeM struct{ code int }

func (m fakeM) Run() int { return m.code }
