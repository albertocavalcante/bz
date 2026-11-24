package publisher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/albertocavalcante/bz/internal/coord"
)

// FilePublisher writes module files to a local filesystem directory.
type FilePublisher struct {
	basePath string
	root     *os.Root // all filesystem access is confined to basePath
}

// NewFilePublisher creates a new filesystem publisher.
// The basePath is the root directory where modules will be written.
func NewFilePublisher(basePath string) (*FilePublisher, error) {
	// Ensure the base path exists or can be created
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("create base path: %w", err)
	}

	// Get absolute path
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute path: %w", err)
	}

	root, err := os.OpenRoot(absPath)
	if err != nil {
		return nil, fmt.Errorf("open base path: %w", err)
	}

	return &FilePublisher{basePath: absPath, root: root}, nil
}

// Put writes content to a file, creating directories as needed.
// The path must be a valid registry path (see coord.ValidateRegistryPath);
// the write is confined to the base directory.
func (p *FilePublisher) Put(_ context.Context, path string, content []byte) error {
	return putInRoot(p.root, path, content)
}

// Exists checks if a file exists at the given path.
func (p *FilePublisher) Exists(_ context.Context, path string) (bool, error) {
	return existsInRoot(p.root, path)
}

// putInRoot validates a registry path and writes content under root,
// creating parent directories. os.Root refuses any path that would resolve
// outside root, including through symbolic links.
func putInRoot(root *os.Root, path string, content []byte) error {
	if err := coord.ValidateRegistryPath(path); err != nil {
		return err
	}
	name := filepath.FromSlash(path)

	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}
	if err := root.WriteFile(name, content, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// existsInRoot validates a registry path and reports whether it exists under root.
func existsInRoot(root *os.Root, path string) (bool, error) {
	if err := coord.ValidateRegistryPath(path); err != nil {
		return false, err
	}

	_, err := root.Stat(filepath.FromSlash(path))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat file: %w", err)
}

// Finalize is a no-op for file publisher.
func (p *FilePublisher) Finalize(_ context.Context, _ string) error {
	return nil
}

// Type returns the publisher type identifier.
func (p *FilePublisher) Type() string {
	return TypeFile
}

// Close releases the handle on the base directory.
func (p *FilePublisher) Close() error {
	if p.root == nil {
		return nil
	}
	err := p.root.Close()
	p.root = nil
	return err
}

// Verify FilePublisher implements Publisher.
var _ Publisher = (*FilePublisher)(nil)
