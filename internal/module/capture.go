package module

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	ast "github.com/albertocavalcante/go-bzlmod-ast"
)

// CaptureWorkspace parses the workspace and retains the exact bytes read for
// the root and includes. The parser bounds reads and reuses parsed includes.
func CaptureWorkspace(ctx context.Context, fsys fs.FS) (*File, map[string][]byte, error) {
	src := captureFS{FS: fsys, sources: make(map[string][]byte)}
	x, err := ast.ParseWithIncludes(ctx, src, ast.IncludeOptions{})
	f, err := loadExpanded(moduleFileName, x, err)
	if err != nil {
		return nil, nil, err
	}
	return f, src.sources, nil
}

// OpenWorkspaceForUpdate opens a disk workspace for editing, retaining the
// originals used for parsing. It preserves the CLI's separately selected root
// file and resolves included files through the same os.Root used for writes.
func OpenWorkspaceForUpdate(ctx context.Context, modulePath string) (*Workspace, map[string][]byte, error) {
	w, err := openWorkspace(modulePath)
	if err != nil {
		return nil, nil, err
	}
	sources, err := w.capture(ctx, modulePath)
	if err != nil {
		_ = w.Close()
		return nil, nil, err
	}
	return w, sources, nil
}

func (w *Workspace) capture(ctx context.Context, modulePath string) (map[string][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(w.rootReal)
	if err != nil {
		return nil, fmt.Errorf("parse error: failed to read %s: %w", modulePath, err)
	}
	result, err := ast.ParseContent(modulePath, content)
	w.File, err = loadParsed(modulePath, result, err)
	if err != nil {
		return nil, err
	}

	src := captureFS{FS: w.root.FS(), sources: map[string][]byte{w.rootRel: content}}
	if len(w.File.Includes) > 0 {
		x, err := ast.ExpandIncludes(ctx, result, src, ast.IncludeOptions{})
		w.File, err = loadExpanded(modulePath, x, err)
		if err != nil {
			return nil, err
		}
	}
	return src.sources, ctx.Err()
}

// captureFS observes bytes as the parser reads them. Forwarding Stat keeps a
// stat-only check from reopening a file on file systems that support it.
type captureFS struct {
	fs.FS
	sources map[string][]byte
}

func (c captureFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(c.FS, name)
}

func (c captureFS) Open(name string) (fs.File, error) {
	f, err := c.FS.Open(name)
	if err != nil {
		return nil, err
	}
	c.sources[name] = nil
	return &captureFile{File: f, name: name, sources: c.sources}, nil
}

type captureFile struct {
	fs.File
	name    string
	sources map[string][]byte
}

func (f *captureFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	f.sources[f.name] = append(f.sources[f.name], p[:n]...)
	return n, err
}
