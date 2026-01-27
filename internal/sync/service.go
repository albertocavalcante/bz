package sync

import (
	"context"
	"fmt"

	"github.com/albertocavalcante/bz/internal/config"
)

// Service orchestrates module syncing from source to destination.
type Service struct {
	cfg *config.Config
}

// NewService creates a new sync service.
func NewService(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// Options for running a sync workflow.
type Options struct {
	DryRun  bool     // Don't actually write, just log what would happen
	Verbose bool     // Print detailed progress
	Modules []string // Override workflow modules (if set, only sync these)
}

// Result contains the sync results.
type Result struct {
	ModulesProcessed int
	ModulesSkipped   int // Already existed
	ModulesWritten   int
	Errors           []error
}

// Run executes a named workflow.
func (s *Service) Run(ctx context.Context, workflowName string, opts Options) (*Result, error) {
	// Find the workflow by name
	var workflow *config.Workflow
	for _, w := range s.cfg.Workflows {
		if w.Name == workflowName {
			workflow = w
			break
		}
	}

	if workflow == nil {
		return nil, fmt.Errorf("workflow %q not found", workflowName)
	}

	// Create and run the runner
	r, err := newRunner(workflow, opts)
	if err != nil {
		return nil, fmt.Errorf("initializing runner: %w", err)
	}
	defer r.close()

	return r.run(ctx)
}

// ListWorkflows returns the names of all defined workflows.
func (s *Service) ListWorkflows() []string {
	names := make([]string, len(s.cfg.Workflows))
	for i, w := range s.cfg.Workflows {
		names[i] = w.Name
	}
	return names
}

// GetWorkflow returns a workflow by name.
func (s *Service) GetWorkflow(name string) *config.Workflow {
	for _, w := range s.cfg.Workflows {
		if w.Name == name {
			return w
		}
	}
	return nil
}
