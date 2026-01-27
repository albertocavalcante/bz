package publisher

import "context"

// Publisher type identifiers.
const (
	TypeFile    = "file"
	TypeHTTPPut = "http_put"
	TypeGit     = "git"
)

// Publisher writes Bazel module files to a destination.
type Publisher interface {
	// Put writes content to a path in BCR structure.
	// Path is relative to the modules directory, e.g., "rules_go/0.50.0/MODULE.bazel"
	Put(ctx context.Context, path string, content []byte) error

	// Exists checks if a path exists in the destination.
	Exists(ctx context.Context, path string) (bool, error)

	// Finalize completes the publishing operation.
	// For git: commits and pushes changes.
	// For file/http: no-op.
	Finalize(ctx context.Context, message string) error

	// Type returns the publisher type for display.
	Type() string

	// Close releases any resources.
	Close() error
}
