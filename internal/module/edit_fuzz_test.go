package module

import (
	"bytes"
	"testing"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/bz/internal/coord"
)

// FuzzSetDepVersion checks that SetDepVersion either refuses and leaves the
// input alone, or changes only the contents of the named dependency's
// version literal.
func FuzzSetDepVersion(f *testing.F) {
	f.Fuzz(func(t *testing.T, src []byte, name, version string) {
		orig := bytes.Clone(src)
		res, _, err := SetDepVersion(src, name, version)
		if !bytes.Equal(src, orig) {
			t.Fatal("input was modified")
		}
		if err != nil {
			return
		}
		if coord.ValidateVersion(version) != nil {
			t.Fatalf("invalid version %q accepted", version)
		}
		inDeps, err := findDeps(src)
		if err != nil {
			t.Fatalf("input does not parse but edit succeeded: %v", err)
		}
		outDeps, err := findDeps(res)
		if err != nil {
			t.Fatalf("result does not parse: %v", err)
		}
		if got, want := stmtCount(t, res), stmtCount(t, src); got != want {
			t.Fatalf("statement count %d, want %d", got, want)
		}
		if got, want := outside(t, res, outDeps, name), outside(t, src, inDeps, name); got != want {
			t.Fatalf("bytes outside the version literal changed:\n got %q\nwant %q", got, want)
		}
		for _, d := range outDeps {
			if d.name == name && d.version.Value != version {
				t.Fatalf("version literal is %q, want %q", d.version.Value, version)
			}
		}
	})
}

func stmtCount(t *testing.T, src []byte) int {
	t.Helper()
	f, err := build.ParseModule("MODULE.bazel", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return len(f.Stmt)
}

// outside returns src with the version literals of every dependency called
// name cut out.
func outside(t *testing.T, src []byte, deps []depStmt, name string) string {
	t.Helper()
	var b bytes.Buffer
	pos := 0
	for _, d := range deps {
		if d.name != name || d.version == nil {
			continue
		}
		s, e := d.version.Span()
		b.Write(src[pos:s.Byte])
		pos = e.Byte
	}
	b.Write(src[pos:])
	return b.String()
}
