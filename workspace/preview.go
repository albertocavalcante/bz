// Package workspace previews edits to a Bazel workspace's MODULE.bazel files.
package workspace

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/module"
)

var (
	// ErrDependencyNotFound means the workspace has no bazel_dep with the requested name.
	ErrDependencyNotFound = module.ErrDepNotFound
	// ErrAmbiguousDependency means the declaration or version argument cannot be uniquely selected.
	ErrAmbiguousDependency = module.ErrAmbiguousDep
	// ErrVersionlessDependency means the dependency has no nonempty version string literal.
	ErrVersionlessDependency = module.ErrNoVersionArgument
	// ErrInvalidModuleName means the requested module name is invalid.
	ErrInvalidModuleName = coord.ErrInvalidModuleName
	// ErrInvalidVersion means the requested version is invalid.
	ErrInvalidVersion = coord.ErrInvalidVersion
)

// Edit contains the whole declaring file before and after the proposed change.
// The caller owns both byte slices, which may be modified independently.
type Edit struct {
	Path           string // workspace-relative, slash-separated file path
	Line, Column   int    // one-based declaration line and rune column
	CurrentVersion string
	Before, After  []byte
	Changed        bool
}

// PreviewVersion proposes replacing the version of one existing bazel_dep,
// looking in MODULE.bazel at the root of fsys and following its include calls.
// A declaration in a file included more than once by the same path is
// considered once. Separate declarations with the same name are ambiguous,
// even within a single file.
// If the version literal already has the requested value, the result has
// equal Before and After bytes and Changed is false.
//
// The result uses the exact bytes read by the bounded workspace parser.
// Files are read in sequence and may reflect different points in time
// if the workspace changes during the call. The function only reads from
// fsys, which controls path confinement. To confine disk reads, open the
// workspace directory with [os.OpenRoot] and pass its FS().
//
// Errors return a zero Edit. Use [errors.Is] to test the package sentinel
// errors, context cancellation, or underlying I/O errors.
func PreviewVersion(ctx context.Context, fsys fs.FS, name, version string) (Edit, error) {
	if err := ctx.Err(); err != nil {
		return Edit{}, err
	}
	if err := module.ValidateDep(name, version); err != nil {
		return Edit{}, err
	}
	file, sources, err := module.CaptureWorkspace(ctx, fsys)
	if err != nil {
		return Edit{}, err
	}
	if err := ctx.Err(); err != nil {
		return Edit{}, err
	}
	decls := file.Declarations(name)
	switch len(decls) {
	case 0:
		return Edit{}, fmt.Errorf("%w: %s", ErrDependencyNotFound, name)
	case 1:
	default:
		return Edit{}, fmt.Errorf("%w: %s has %d declarations", ErrAmbiguousDependency, name, len(decls))
	}

	origin := decls[0]
	before := sources[origin.File]
	after, current, err := module.PreviewVersionAt(before, name, version, origin)
	if err != nil {
		return Edit{}, fmt.Errorf("preview %s: %w", origin.File, err)
	}
	if err := ctx.Err(); err != nil {
		return Edit{}, err
	}
	return Edit{
		Path:           origin.File,
		Line:           origin.Pos.Start.Line,
		Column:         origin.Pos.Start.Column,
		CurrentVersion: current,
		Before:         before,
		After:          after,
		Changed:        current != version,
	}, nil
}
