package module

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ast "github.com/albertocavalcante/go-bzlmod-ast"
)

// Workspace is a loaded MODULE.bazel together with the directory it lives in.
// Included files are read and written only through an os.Root on that
// directory, so an edit can neither leave the workspace nor follow a symlink
// out of it.
type Workspace struct {
	File *File

	dir  string
	root *os.Root
}

// OpenWorkspace loads modulePath with its include() calls expanded and opens
// its directory for editing. The caller must Close it.
func OpenWorkspace(modulePath string) (*Workspace, error) {
	f, err := Load(modulePath)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(modulePath)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open workspace root: %w", err)
	}
	return &Workspace{File: f, dir: dir, root: root}, nil
}

// Close releases the workspace root.
func (w *Workspace) Close() error { return w.root.Close() }

// RelPath returns the workspace-relative, slash-separated path of the file
// that declares o.
func (w *Workspace) RelPath(o Origin) string {
	if o.IsRoot() {
		return filepath.Base(w.File.Path)
	}
	return o.File
}

// ReadFile returns the bytes of a workspace-relative file.
func (w *Workspace) ReadFile(rel string) ([]byte, error) {
	return w.root.ReadFile(filepath.FromSlash(rel))
}

// Declarations returns where each bazel_dep named name is declared, in
// evaluation order. A file included more than once is reported once.
func (f *File) Declarations(name string) []Origin {
	var out []Origin
	seen := map[*ast.BazelDep]bool{}
	for _, d := range f.Deps {
		if d.Name.String() != name || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, f.OriginOf(d))
	}
	return out
}

// UniqueDeps returns Deps without the repeats a file included more than once
// produces.
func (f *File) UniqueDeps() []*ast.BazelDep {
	seen := map[*ast.BazelDep]bool{}
	out := make([]*ast.BazelDep, 0, len(f.Deps))
	for _, d := range f.Deps {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

// CheckDeclaredOnce returns an error listing every declaration when more than
// one file declares name, because editing only one of them would silently
// leave the others behind. A name declared nowhere is not an error.
func (w *Workspace) CheckDeclaredOnce(name string) error {
	decls := w.File.Declarations(name)
	files := map[string]bool{}
	where := make([]string, 0, len(decls))
	for _, o := range decls {
		files[w.RelPath(o)] = true
		where = append(where, fmt.Sprintf("%s:%d", w.RelPath(o), o.Pos.Start.Line))
	}
	if len(files) > 1 {
		return fmt.Errorf("%s is declared more than once (%s); remove the duplicate declarations by hand first", name, strings.Join(where, ", "))
	}
	return nil
}

// AlreadyDeclaredError reports a bazel_dep that exists in the root or an
// included file.
type AlreadyDeclaredError struct {
	Name string
	File string
	Line int
}

func (e *AlreadyDeclaredError) Error() string {
	return fmt.Sprintf("%s is already declared in %s:%d", e.Name, e.File, e.Line)
}

// AddDeps appends deps to the root MODULE.bazel, using its line endings. It
// refuses a module that is already declared in the root or any included file.
func (w *Workspace) AddDeps(deps []DepSpec) error {
	for _, d := range deps {
		if decls := w.File.Declarations(d.Name); len(decls) > 0 {
			return &AlreadyDeclaredError{Name: d.Name, File: w.RelPath(decls[0]), Line: decls[0].Pos.Start.Line}
		}
	}
	rel := filepath.Base(w.File.Path)
	src, err := w.ReadFile(rel)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", rel, err)
	}
	updated, err := AppendDeps(src, deps)
	if err != nil {
		return fmt.Errorf("failed to add to %s: %w", rel, err)
	}
	return w.WriteAll([]FileEdit{{Rel: rel, Original: src, Updated: updated}})
}

// FileEdit is the new content of one workspace file, computed from Original.
type FileEdit struct {
	Rel               string // workspace-relative, slash-separated
	Original, Updated []byte
}

// PartialWriteError reports a multi-file write that failed after some files
// had already been replaced.
type PartialWriteError struct {
	Failed     string
	Err        error
	Written    []string
	NotWritten []string
}

func (e *PartialWriteError) Error() string {
	return fmt.Sprintf("failed to write %s: %v; files written: %s; files NOT written: %s",
		e.Failed, e.Err, strings.Join(e.Written, ", "), strings.Join(e.NotWritten, ", "))
}

func (e *PartialWriteError) Unwrap() error { return e.Err }

// WriteAll replaces every edited file. Nothing is written unless every file
// still has the bytes the edit was computed from and can be written; then each
// file is replaced atomically, one after the other. A failure in that second
// phase (disk full, a concurrent change) cannot be rolled back: it returns a
// *PartialWriteError naming exactly which files were and were not written.
func (w *Workspace) WriteAll(edits []FileEdit) error {
	todo := make([]FileEdit, 0, len(edits))
	for _, e := range edits {
		if !bytes.Equal(e.Original, e.Updated) {
			todo = append(todo, e)
		}
	}
	for _, e := range todo {
		cur, err := w.ReadFile(e.Rel)
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", e.Rel, err)
		}
		if !bytes.Equal(cur, e.Original) {
			return fmt.Errorf("failed to write %s: %w", e.Rel, ErrFileChanged)
		}
		// Replacing a read-only file by rename would bypass its permissions.
		probe, err := w.root.OpenFile(filepath.FromSlash(e.Rel), os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", e.Rel, err)
		}
		_ = probe.Close()
	}
	for i, e := range todo {
		if err := w.writeAtomic(e.Rel, e.Updated); err != nil {
			if i == 0 {
				return fmt.Errorf("failed to write %s: %w", e.Rel, err)
			}
			perr := &PartialWriteError{Failed: e.Rel, Err: err}
			for _, d := range todo[:i] {
				perr.Written = append(perr.Written, d.Rel)
			}
			for _, d := range todo[i:] {
				perr.NotWritten = append(perr.NotWritten, d.Rel)
			}
			return perr
		}
	}
	return nil
}

// writeAtomic replaces rel through the workspace root with a temporary file
// in the same directory, keeping the file mode. A symlink is followed only
// when it stays inside the workspace.
func (w *Workspace) writeAtomic(rel string, data []byte) error {
	rel = filepath.FromSlash(rel)
	if info, err := w.root.Lstat(rel); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		resolved, err := w.resolveInside(rel)
		if err != nil {
			return err
		}
		rel = resolved
	}
	info, err := w.root.Stat(rel)
	if err != nil {
		return err
	}
	tmpRel := filepath.Join(filepath.Dir(rel), ".bz-"+filepath.Base(rel)+"."+strconv.Itoa(os.Getpid())+"."+strconv.FormatInt(time.Now().UnixNano(), 36))
	tmp, err := w.root.OpenFile(tmpRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if err := writeAndClose(tmp, data, info.Mode().Perm()); err != nil {
		_ = w.root.Remove(tmpRel)
		return err
	}
	if err := w.root.Rename(tmpRel, rel); err != nil {
		_ = w.root.Remove(tmpRel)
		return err
	}
	return nil
}

func (w *Workspace) resolveInside(rel string) (string, error) {
	base, err := filepath.EvalSymlinks(w.dir)
	if err != nil {
		return "", err
	}
	target, err := filepath.EvalSymlinks(filepath.Join(w.dir, rel))
	if err != nil {
		return "", err
	}
	inner, err := filepath.Rel(base, target)
	if err != nil || inner == ".." || strings.HasPrefix(inner, ".."+string(filepath.Separator)) {
		return "", errors.New("refusing to write through a symlink that leaves the workspace")
	}
	return inner, nil
}
