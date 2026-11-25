package publisher

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/registry"
)

// FilePublisher writes module files to a local filesystem directory.
type FilePublisher struct {
	root           *os.Root
	registryLayout bool
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

	return &FilePublisher{root: root}, nil
}

// NewFileRegistryPublisher writes module-relative paths inside the modules
// directory of a local registry. That directory is created on the first write.
func NewFileRegistryPublisher(basePath string) (*FilePublisher, error) {
	p, err := NewFilePublisher(basePath)
	if err != nil {
		return nil, err
	}
	p.registryLayout = true
	return p, nil
}

// OpenFileRegistryPublisher opens an existing registry without creating it.
// A missing registry is treated as empty, for dry runs.
func OpenFileRegistryPublisher(basePath string) (*FilePublisher, error) {
	root, err := os.OpenRoot(basePath)
	if errors.Is(err, fs.ErrNotExist) {
		return &FilePublisher{registryLayout: true}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open base path: %w", err)
	}
	return &FilePublisher{root: root, registryLayout: true}, nil
}

// moduleRoot opens a confined root for module-relative access. The returned
// release function closes only temporary handles, leaving p.root open.
func (p *FilePublisher) moduleRoot(create bool) (*os.Root, func(), error) {
	if p.root == nil {
		return nil, nil, fs.ErrNotExist
	}
	if !p.registryLayout {
		return p.root, func() {}, nil
	}
	if create {
		if err := p.root.MkdirAll(registry.ModulesDir, 0o755); err != nil {
			return nil, nil, err
		}
	}
	root, err := p.root.OpenRoot(registry.ModulesDir)
	if err != nil {
		return nil, nil, err
	}
	return root, func() { _ = root.Close() }, nil
}

// Put writes content to a module-relative path, creating directories as needed.
func (p *FilePublisher) Put(_ context.Context, path string, content []byte) error {
	if err := coord.ValidateRegistryPath(path); err != nil {
		return err
	}
	root, release, err := p.moduleRoot(true)
	if err != nil {
		return err
	}
	defer release()
	return putInRoot(root, path, content)
}

// Exists checks if a module-relative path exists.
func (p *FilePublisher) Exists(_ context.Context, path string) (bool, error) {
	if err := coord.ValidateRegistryPath(path); err != nil {
		return false, err
	}
	root, release, err := p.moduleRoot(false)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer release()
	return existsInRoot(root, path)
}

// Read retrieves a published file relative to the modules directory.
func (p *FilePublisher) Read(path string) ([]byte, error) {
	if err := coord.ValidateRegistryPath(path); err != nil {
		return nil, err
	}
	root, release, err := p.moduleRoot(false)
	if err != nil {
		return nil, err
	}
	defer release()
	return root.ReadFile(filepath.FromSlash(path))
}

// Replace atomically replaces a file in an existing module directory.
func (p *FilePublisher) Replace(path string, content []byte) error {
	if err := coord.ValidateRegistryPath(path); err != nil {
		return err
	}
	root, release, err := p.moduleRoot(false)
	if err != nil {
		return err
	}
	defer release()

	name := filepath.FromSlash(path)
	mode := fs.FileMode(0o644)
	info, err := root.Lstat(name)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			return fmt.Errorf("cannot replace non-regular file %q", path)
		}
		mode = info.Mode().Perm()
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}

	temp := filepath.Join(filepath.Dir(name), ".bz-"+rand.Text())
	file, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temp) }()
	_, writeErr := file.Write(content)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return root.Rename(temp, name)
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
