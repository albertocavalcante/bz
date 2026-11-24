package publisher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitPublisher writes module files to a git repository.
type GitPublisher struct {
	repoURL    string
	branch     string
	workDir    string   // temp directory with cloned repo
	root       *os.Root // confines file access to workDir
	hasChanges bool
}

// NewGitPublisher creates a new git repository publisher.
// It clones the repository to a temporary directory.
func NewGitPublisher(repoURL, branch string) (*GitPublisher, error) {
	if repoURL == "" {
		return nil, fmt.Errorf("repository URL is required")
	}
	if branch == "" {
		branch = "main"
	}

	// Create temp directory
	workDir, err := os.MkdirTemp("", "bz-git-publisher-*")
	if err != nil {
		return nil, fmt.Errorf("create temp directory: %w", err)
	}

	p := &GitPublisher{
		repoURL: repoURL,
		branch:  branch,
		workDir: workDir,
	}

	// Clone the repository
	if err := p.clone(); err != nil {
		// Clean up on failure
		_ = os.RemoveAll(workDir)
		return nil, err
	}

	root, err := os.OpenRoot(workDir)
	if err != nil {
		_ = os.RemoveAll(workDir)
		return nil, fmt.Errorf("open work directory: %w", err)
	}
	p.root = root

	return p, nil
}

// Put writes content to a file in the cloned repository.
// The path must be a valid registry path (see coord.ValidateRegistryPath);
// the write is confined to the clone.
func (p *GitPublisher) Put(_ context.Context, path string, content []byte) error {
	if p.root == nil {
		return errors.New("git publisher is closed")
	}
	if err := putInRoot(p.root, path, content); err != nil {
		return err
	}
	p.hasChanges = true
	return nil
}

// Exists checks if a file exists in the cloned repository.
func (p *GitPublisher) Exists(_ context.Context, path string) (bool, error) {
	if p.root == nil {
		return false, errors.New("git publisher is closed")
	}
	return existsInRoot(p.root, path)
}

// Finalize commits and pushes all changes.
func (p *GitPublisher) Finalize(ctx context.Context, message string) error {
	if !p.hasChanges {
		return nil
	}

	if message == "" {
		message = "Update module files"
	}

	// Stage all changes
	if err := p.runGitCommand(ctx, "add", "."); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Check if there are staged changes
	// diff --cached --quiet exits with 0 if no changes, 1 if changes
	if err := p.runGitCommand(ctx, "diff", "--cached", "--quiet"); err == nil {
		// No changes to commit
		return nil
	}

	// Commit
	if err := p.runGitCommand(ctx, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Push
	if err := p.runGitCommand(ctx, "push"); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// Type returns the publisher type identifier.
func (p *GitPublisher) Type() string {
	return TypeGit
}

// Close removes the temporary directory.
func (p *GitPublisher) Close() error {
	if p.root != nil {
		_ = p.root.Close()
		p.root = nil
	}
	if p.workDir != "" {
		return os.RemoveAll(p.workDir)
	}
	return nil
}

// clone clones the repository to the work directory.
func (p *GitPublisher) clone() error {
	ctx := context.Background()

	args := []string{"clone", "--depth", "1", "--branch", p.branch, "--", p.repoURL, p.workDir}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	return nil
}

// runGitCommand runs a git command and returns an error if it fails.
// The stdout is discarded; only stderr is captured for error messages.
func (p *GitPublisher) runGitCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = p.workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// Verify GitPublisher implements Publisher.
var _ Publisher = (*GitPublisher)(nil)
