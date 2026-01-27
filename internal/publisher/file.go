package publisher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FilePublisher writes module files to a local filesystem directory.
type FilePublisher struct {
	basePath string
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

	return &FilePublisher{basePath: absPath}, nil
}

// Put writes content to a file, creating directories as needed.
func (p *FilePublisher) Put(_ context.Context, path string, content []byte) error {
	fullPath := filepath.Join(p.basePath, path)

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	// Write the file
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Exists checks if a file exists at the given path.
func (p *FilePublisher) Exists(_ context.Context, path string) (bool, error) {
	fullPath := filepath.Join(p.basePath, path)

	_, err := os.Stat(fullPath)
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

// Close releases any resources (no-op for file publisher).
func (p *FilePublisher) Close() error {
	return nil
}

// Verify FilePublisher implements Publisher.
var _ Publisher = (*FilePublisher)(nil)
