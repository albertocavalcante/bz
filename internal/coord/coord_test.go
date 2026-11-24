package coord

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestValidateModuleName(t *testing.T) {
	t.Parallel()

	accepted := []string{
		"a", "z9", "abseil-cpp", "rules_go", "boringssl", "bazel_skylib", "re2",
		"protobuf", "rules_cc", "apple_support", "a.b", "a-b_c.d1", "googletest",
	}
	rejected := []string{
		"", ".", "..", "../x", "../../victim", "a/b", `a\b`, "/abs", "Rules_go",
		"rules_go/", "1abc", "_abc", "-abc", ".abc", "abc-", "abc.", "abc_",
		"a b", "a\x00b", "@scope/pkg", "a@1", "abc\n", "ä",
	}

	for _, name := range accepted {
		if err := ValidateModuleName(name); err != nil {
			t.Errorf("ValidateModuleName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range rejected {
		err := ValidateModuleName(name)
		if !errors.Is(err, ErrInvalidModuleName) {
			t.Errorf("ValidateModuleName(%q) = %v, want ErrInvalidModuleName", name, err)
		}
	}
}

func TestValidateVersion(t *testing.T) {
	t.Parallel()

	accepted := []string{
		// Shapes published on the Bazel Central Registry.
		"1.2.3.bcr.1", "0.0.0-20241220-5e258e3", "20240116.2", "1.0.0-rc1",
		"37.0-rc3", "1.3.1.bcr.3", "0.0.10", "2024-07-02.bcr.1", "20260817.0",
		"0.20261105.0", "36.0.bcr.1",
		// Other shapes Bazel's Version.parse accepts.
		"1", "1.0", "v1.0", "1.0.0+build.5", "1.0.0-alpha.1+meta-data", "1.0+..",
		"1.0.0-rc.1", "18446744073709551615",
	}
	rejected := []string{
		"", ".", "..", "...", "../../../esc", "1.0/../..", "1.0/x", `1.0\x`, "/1.0",
		"1..0", ".1", "1.", "-1", "1.0-", "1.0-.", "1.0-a..b", "1.0+", "+1",
		"1.0_0", "1.0 ", "1.0\x00", "1.0:", "1.0%2f", "18446744073709551616",
	}

	for _, v := range accepted {
		if err := ValidateVersion(v); err != nil {
			t.Errorf("ValidateVersion(%q) = %v, want nil", v, err)
		}
	}
	for _, v := range rejected {
		err := ValidateVersion(v)
		if !errors.Is(err, ErrInvalidVersion) {
			t.Errorf("ValidateVersion(%q) = %v, want ErrInvalidVersion", v, err)
		}
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	if err := Validate("rules_go", "0.50.1"); err != nil {
		t.Fatalf("Validate = %v", err)
	}
	if err := Validate("..", "1.0"); !errors.Is(err, ErrInvalidModuleName) {
		t.Fatalf("Validate bad name = %v", err)
	}
	if err := Validate("rules_go", ".."); !errors.Is(err, ErrInvalidVersion) {
		t.Fatalf("Validate bad version = %v", err)
	}
}

func TestValidateRegistryPath(t *testing.T) {
	t.Parallel()

	accepted := []string{
		"rules_go/metadata.json",
		"rules_go/0.50.0/MODULE.bazel",
		"rules_go/0.50.0/source.json",
		"protobuf/37.0-rc3/patches/fix.patch",
	}
	rejected := []string{
		"", "rules_go", "/rules_go/1.0/MODULE.bazel", "../rules_go/1.0/MODULE.bazel",
		"rules_go/../../../esc/MODULE.bazel", "rules_go/./MODULE.bazel",
		"rules_go//MODULE.bazel", `rules_go\1.0\MODULE.bazel`, "rules_go/1.0/../../x",
		"Bad/1.0/MODULE.bazel", "rules_go/1..0/MODULE.bazel", "rules_go/1.0/",
		"rules_go/1.0/x\x00y",
	}

	for _, p := range accepted {
		if err := ValidateRegistryPath(p); err != nil {
			t.Errorf("ValidateRegistryPath(%q) = %v, want nil", p, err)
		}
	}
	for _, p := range rejected {
		if err := ValidateRegistryPath(p); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("ValidateRegistryPath(%q) = %v, want ErrInvalidPath", p, err)
		}
	}
}

func TestJoinAndWithin(t *testing.T) {
	t.Parallel()

	base := t.TempDir()

	got, err := Join(base, "rules_go", "1.0", "MODULE.bazel")
	if err != nil {
		t.Fatalf("Join = %v", err)
	}
	if want := filepath.Join(base, "rules_go", "1.0", "MODULE.bazel"); got != want {
		t.Fatalf("Join = %q, want %q", got, want)
	}

	escapes := [][]string{
		{".."},
		{"../../victim"},
		{"a", "..", ".."},
		{""},
		{"."},
		{"a", ".."},
	}
	for _, elems := range escapes {
		if _, err := Join(base, elems...); !errors.Is(err, ErrEscapesBase) {
			t.Errorf("Join(base, %q) = %v, want ErrEscapesBase", elems, err)
		}
	}

	if err := Within(base, base); err != nil {
		t.Errorf("Within(base, base) = %v, want nil", err)
	}
	if err := Within(base, filepath.Dir(base)); !errors.Is(err, ErrEscapesBase) {
		t.Errorf("Within(parent) = %v, want ErrEscapesBase", err)
	}
	// A sibling sharing the base as a string prefix is outside.
	if err := Within(base, base+"-sibling"); !errors.Is(err, ErrEscapesBase) {
		t.Errorf("Within(sibling) = %v, want ErrEscapesBase", err)
	}
	// A name that merely starts with ".." stays inside.
	if err := Within(base, filepath.Join(base, "..foo")); err != nil {
		t.Errorf("Within(..foo) = %v, want nil", err)
	}
}
