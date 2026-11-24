package module

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/bazelbuild/buildtools/build"
)

// ErrNoVersionArgument indicates that a bazel_dep has no literal version
// argument to edit (for example when an override supplies the version).
var ErrNoVersionArgument = errors.New("bazel_dep has no version argument")

// ErrDepNotFound indicates that no bazel_dep with the requested name exists.
var ErrDepNotFound = errors.New("bazel_dep not found")

// DepSpec describes a bazel_dep statement to append.
type DepSpec struct {
	Name    string
	Version string
	Dev     bool
}

// depStmt is a top-level bazel_dep() call with its byte span in the source.
type depStmt struct {
	name       string
	start, end int               // byte span of the whole call expression
	version    *build.StringExpr // nil when there is no literal version argument
}

// findDeps parses src with buildtools and returns every top-level
// bazel_dep() call whose name argument is a string literal. Spans come from
// the real syntax tree, so strings and comments containing parentheses are
// handled correctly.
func findDeps(src []byte) ([]depStmt, error) {
	f, err := build.ParseModule("MODULE.bazel", src)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	var deps []depStmt
	for _, stmt := range f.Stmt {
		call, ok := stmt.(*build.CallExpr)
		if !ok {
			continue
		}
		if id, ok := call.X.(*build.Ident); !ok || id.Name != "bazel_dep" {
			continue
		}
		d := depStmt{}
		haveName := false
		for _, arg := range call.List {
			as, ok := arg.(*build.AssignExpr)
			if !ok {
				continue
			}
			key, ok := as.LHS.(*build.Ident)
			if !ok {
				continue
			}
			val, ok := as.RHS.(*build.StringExpr)
			if !ok {
				continue
			}
			switch key.Name {
			case "name":
				d.name = val.Value
				haveName = true
			case "version":
				d.version = val
			}
		}
		if !haveName {
			continue
		}
		s, e := call.Span()
		d.start, d.end = s.Byte, e.Byte
		deps = append(deps, d)
	}
	return deps, nil
}

// validate checks that src still parses as a MODULE.bazel file.
func validate(src []byte) error {
	if _, err := LoadContent("MODULE.bazel", src); err != nil {
		return fmt.Errorf("edit would produce an invalid MODULE.bazel: %w", err)
	}
	return nil
}

// DetectEOL returns "\r\n" when CRLF is the dominant line ending in src and
// "\n" otherwise (including when src has no line endings).
func DetectEOL(src []byte) string {
	crlf := bytes.Count(src, []byte("\r\n"))
	lf := bytes.Count(src, []byte("\n")) - crlf
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}

// RemoveDeps removes every bazel_dep statement named in names, including the
// full multi-line span, the rest of its last line (a trailing comment) and
// the line terminator. Everything else is preserved byte for byte. The result
// is re-parsed; an unparsable result is an error. It returns the names that
// were removed; a requested name that cannot be located is an error and
// nothing is returned.
func RemoveDeps(src []byte, names []string) ([]byte, []string, error) {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	deps, err := findDeps(src)
	if err != nil {
		return nil, nil, err
	}

	type span struct{ start, end int }
	var spans []span
	located := make(map[string]bool)
	for _, d := range deps {
		if want[d.name] {
			spans = append(spans, removalSpan(src, d.start, d.end))
			located[d.name] = true
		}
	}
	var removed, missing []string
	for _, n := range names {
		switch {
		case !located[n]:
			missing = append(missing, n)
		case !slices.Contains(removed, n):
			removed = append(removed, n)
		}
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrDepNotFound, strings.Join(missing, ", "))
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	var out bytes.Buffer
	pos := 0
	for _, s := range spans {
		out.Write(src[pos:s.start])
		pos = s.end
	}
	out.Write(src[pos:])
	res := out.Bytes()

	// Preserve a missing final newline.
	if len(src) > 0 && src[len(src)-1] != '\n' {
		res = bytes.TrimSuffix(bytes.TrimSuffix(res, []byte("\n")), []byte("\r"))
	}
	if err := validate(res); err != nil {
		return nil, nil, err
	}
	return res, removed, nil
}

// removalSpan widens a statement span to whole lines when the statement is
// alone on its lines, swallowing a trailing comment and the line terminator.
func removalSpan(src []byte, start, end int) (s struct{ start, end int }) {
	s.start, s.end = start, end

	e := end
	for e < len(src) && (src[e] == ' ' || src[e] == '\t') {
		e++
	}
	if e < len(src) && src[e] == '#' {
		for e < len(src) && src[e] != '\n' && src[e] != '\r' {
			e++
		}
	}
	switch {
	case e >= len(src):
	case src[e] == '\n':
		e++
	case src[e] == '\r' && e+1 < len(src) && src[e+1] == '\n':
		e += 2
	default:
		return s // other code follows on the same line: remove only the call
	}

	b := start
	for b > 0 && (src[b-1] == ' ' || src[b-1] == '\t') {
		b--
	}
	if b > 0 && src[b-1] != '\n' {
		return s // other code precedes on the same line
	}
	s.start, s.end = b, e
	return s
}

// SetDepVersion replaces the version string literal of every bazel_dep named
// name. It reports whether the bytes changed. ErrDepNotFound and
// ErrNoVersionArgument are returned (wrapped) when there is nothing to edit.
func SetDepVersion(src []byte, name, version string) ([]byte, bool, error) {
	deps, err := findDeps(src)
	if err != nil {
		return nil, false, err
	}

	var targets []depStmt
	for _, d := range deps {
		if d.name == name {
			targets = append(targets, d)
		}
	}
	if len(targets) == 0 {
		return nil, false, fmt.Errorf("%w: %s", ErrDepNotFound, name)
	}
	for _, d := range targets {
		if d.version == nil {
			return nil, false, fmt.Errorf("%w: %s", ErrNoVersionArgument, name)
		}
	}

	res := append([]byte(nil), src...)
	// Apply from the end so earlier offsets stay valid.
	for i := len(targets) - 1; i >= 0; i-- {
		lit := targets[i].version
		s, e := lit.Span()
		q := byte('"')
		if s.Byte < len(res) && (res[s.Byte] == '"' || res[s.Byte] == '\'') {
			q = res[s.Byte]
		}
		repl := string(q) + version + string(q)
		if lit.TripleQuote {
			repl = string(bytes.Repeat([]byte{q}, 3)) + version + string(bytes.Repeat([]byte{q}, 3))
		}
		res = append(res[:s.Byte:s.Byte], append([]byte(repl), res[e.Byte:]...)...)
	}
	if bytes.Equal(res, src) {
		return res, false, nil
	}
	if err := validate(res); err != nil {
		return nil, false, err
	}
	return res, true, nil
}

// AppendDeps appends bazel_dep statements to src using src's dominant line
// ending. A missing final newline stays missing.
func AppendDeps(src []byte, deps []DepSpec) ([]byte, error) {
	if len(deps) == 0 {
		return append([]byte(nil), src...), nil
	}
	eol := DetectEOL(src)
	hadFinalNewline := len(src) == 0 || src[len(src)-1] == '\n'

	var out bytes.Buffer
	out.Write(src)
	if !hadFinalNewline {
		out.WriteString(eol)
	}
	for _, d := range deps {
		out.WriteString(FormatBazelDepEOL(d.Name, d.Version, d.Dev, eol))
	}
	res := out.Bytes()
	if !hadFinalNewline {
		res = bytes.TrimSuffix(bytes.TrimSuffix(res, []byte("\n")), []byte("\r"))
	}
	if err := validate(res); err != nil {
		return nil, err
	}
	return res, nil
}

// FormatBazelDepEOL is FormatBazelDep with an explicit line terminator.
func FormatBazelDepEOL(name, version string, dev bool, eol string) string {
	line := FormatBazelDep(name, version, dev)
	return line[:len(line)-1] + eol
}

// WriteFileAtomic replaces path with data by writing a temporary file in the
// same directory, syncing it and renaming it over the target. The existing
// file mode is preserved (0o644 for a new file); symlinks are followed so the
// link itself survives. On any failure the original file is left untouched.
func WriteFileAtomic(path string, data []byte) error {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
		// Renaming over a read-only file would bypass its permissions, so
		// refuse unless the current user could have written it in place.
		probe, perr := os.OpenFile(target, os.O_WRONLY, 0)
		if perr != nil {
			return perr
		}
		_ = probe.Close()
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".bz-"+filepath.Base(target)+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if err := writeAndClose(tmp, data, mode); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// writeAndClose writes data to f, applies mode, syncs and closes it. The file
// is closed even when an earlier step fails.
func writeAndClose(f *os.File, data []byte, mode os.FileMode) error {
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ErrFileChanged indicates the file was modified after it was read.
var ErrFileChanged = errors.New("MODULE.bazel changed while bz was running; re-run")

// WriteFileIfUnchanged is WriteFileAtomic guarded against lost updates: it
// re-reads path and writes only when its content still equals original, the
// bytes the edit was computed from. Otherwise it returns ErrFileChanged and
// writes nothing.
func WriteFileIfUnchanged(path string, original, data []byte) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return ErrFileChanged
	}
	return WriteFileAtomic(path, data)
}

// OverridesFor returns the overrides that target the named module.
func (f *File) OverridesFor(name string) []Override {
	var out []Override
	for _, o := range f.Overrides {
		if o.ModuleName().String() == name {
			out = append(out, overrideToSummary(o))
		}
	}
	return out
}
