package modsync

import (
	"context"
	"testing"

	"github.com/albertocavalcante/bz/internal/config"
	"github.com/albertocavalcante/bz/internal/publisher"
	"github.com/albertocavalcante/bz/internal/registry"
	"github.com/albertocavalcante/bz/internal/testutil"
)

func TestNewService(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Workflows: []*config.Workflow{
			{Name: "test-workflow"},
		},
	}

	svc := NewService(cfg)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}

	if svc.cfg != cfg {
		t.Error("Service config not set correctly")
	}
}

func TestService_ListWorkflows(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Workflows: []*config.Workflow{
			{Name: "workflow-a"},
			{Name: "workflow-b"},
			{Name: "workflow-c"},
		},
	}

	svc := NewService(cfg)
	names := svc.ListWorkflows()

	if len(names) != 3 {
		t.Errorf("ListWorkflows() returned %d names, want 3", len(names))
	}

	expected := map[string]bool{"workflow-a": true, "workflow-b": true, "workflow-c": true}
	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected workflow name: %s", name)
		}
	}
}

func TestService_ListWorkflows_Empty(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	svc := NewService(cfg)
	names := svc.ListWorkflows()

	if len(names) != 0 {
		t.Errorf("ListWorkflows() returned %d names, want 0", len(names))
	}
}

func TestService_GetWorkflow(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Workflows: []*config.Workflow{
			{Name: "workflow-a", Description: "First workflow"},
			{Name: "workflow-b", Description: "Second workflow"},
		},
	}

	svc := NewService(cfg)

	// Found
	w := svc.GetWorkflow("workflow-a")
	if w == nil {
		t.Fatal("GetWorkflow returned nil for existing workflow")
	}
	if w.Description != "First workflow" {
		t.Errorf("GetWorkflow returned wrong workflow: %s", w.Description)
	}

	// Not found
	w = svc.GetWorkflow("nonexistent")
	if w != nil {
		t.Error("GetWorkflow should return nil for nonexistent workflow")
	}
}

func TestService_Run_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Workflows: []*config.Workflow{
			{Name: "existing"},
		},
	}

	svc := NewService(cfg)
	_, err := svc.Run(context.Background(), "nonexistent", Options{})

	if err == nil {
		t.Error("Run should return error for nonexistent workflow")
	}
}

// TestRunner tests the runner with a file-based setup
func TestRunner_FilterVersions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		versions *config.VersionSelector
		meta     *registry.Metadata
		want     []string
	}{
		{
			name:     "nil selector returns latest",
			versions: nil,
			meta: &registry.Metadata{
				Versions: []string{"1.0.0", "1.1.0", "1.2.0"},
			},
			want: []string{"1.2.0"},
		},
		{
			name: "latest(1) returns last version",
			versions: &config.VersionSelector{
				Type:  "latest",
				Count: 1,
			},
			meta: &registry.Metadata{
				Versions: []string{"1.0.0", "1.1.0", "1.2.0"},
			},
			want: []string{"1.2.0"},
		},
		{
			name: "latest(2) returns last 2 versions",
			versions: &config.VersionSelector{
				Type:  "latest",
				Count: 2,
			},
			meta: &registry.Metadata{
				Versions: []string{"1.0.0", "1.1.0", "1.2.0"},
			},
			want: []string{"1.1.0", "1.2.0"},
		},
		{
			name: "all returns all versions",
			versions: &config.VersionSelector{
				Type: "all",
			},
			meta: &registry.Metadata{
				Versions: []string{"1.0.0", "1.1.0", "1.2.0"},
			},
			want: []string{"1.0.0", "1.1.0", "1.2.0"},
		},
		{
			name: "range with min only",
			versions: &config.VersionSelector{
				Type:   "range",
				MinVer: "1.1.0",
			},
			meta: &registry.Metadata{
				Versions: []string{"1.0.0", "1.1.0", "1.2.0"},
			},
			want: []string{"1.1.0", "1.2.0"},
		},
		{
			name: "range with max only",
			versions: &config.VersionSelector{
				Type:   "range",
				MaxVer: "1.1.0",
			},
			meta: &registry.Metadata{
				Versions: []string{"1.0.0", "1.1.0", "1.2.0"},
			},
			want: []string{"1.0.0", "1.1.0"},
		},
		{
			name: "range with min and max",
			versions: &config.VersionSelector{
				Type:   "range",
				MinVer: "1.0.5",
				MaxVer: "1.1.5",
			},
			meta: &registry.Metadata{
				Versions: []string{"1.0.0", "1.1.0", "1.2.0"},
			},
			want: []string{"1.1.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &runner{
				workflow: &config.Workflow{
					Versions: tt.versions,
				},
			}

			got := r.filterVersions(tt.meta)

			if len(got) != len(tt.want) {
				t.Errorf("filterVersions() returned %d versions, want %d", len(got), len(tt.want))
				t.Errorf("got: %v, want: %v", got, tt.want)
				return
			}

			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("filterVersions()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestRunner_HasTransform(t *testing.T) {
	t.Parallel()
	r := &runner{
		workflow: &config.Workflow{
			Transformations: []*config.Transform{
				{Type: "skip_yanked"},
				{Type: "rewrite_urls", Pattern: "test"},
			},
		},
	}

	if !r.hasTransform(config.Transform{Type: "skip_yanked"}) {
		t.Error("hasTransform should return true for skip_yanked")
	}

	if !r.hasTransform(config.Transform{Type: "rewrite_urls"}) {
		t.Error("hasTransform should return true for rewrite_urls")
	}

	if r.hasTransform(config.Transform{Type: "include_patches"}) {
		t.Error("hasTransform should return false for include_patches")
	}
}

func TestLatestN(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		versions []string
		n        int
		want     []string
	}{
		{
			name:     "n=1",
			versions: []string{"1.0", "2.0", "3.0"},
			n:        1,
			want:     []string{"3.0"},
		},
		{
			name:     "n=2",
			versions: []string{"1.0", "2.0", "3.0"},
			n:        2,
			want:     []string{"2.0", "3.0"},
		},
		{
			name:     "n greater than length",
			versions: []string{"1.0", "2.0"},
			n:        5,
			want:     []string{"1.0", "2.0"},
		},
		{
			name:     "empty slice",
			versions: []string{},
			n:        1,
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := latestN(tt.versions, tt.n)

			if len(got) != len(tt.want) {
				t.Errorf("latestN() returned %d items, want %d", len(got), len(tt.want))
				return
			}

			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("latestN()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestFilterRange(t *testing.T) {
	t.Parallel()
	versions := []string{"1.0.0", "1.1.0", "1.2.0", "1.10.0", "2.0.0"}

	tests := []struct {
		name   string
		min    string
		max    string
		expect []string
	}{
		{
			name:   "min only",
			min:    "1.1.0",
			max:    "",
			expect: []string{"1.1.0", "1.2.0", "1.10.0", "2.0.0"},
		},
		{
			name:   "max only",
			min:    "",
			max:    "1.2.0",
			expect: []string{"1.0.0", "1.1.0", "1.2.0"},
		},
		{
			name:   "both",
			min:    "1.1.0",
			max:    "1.2.0",
			expect: []string{"1.1.0", "1.2.0"},
		},
		{
			name:   "semantic ordering",
			min:    "1.2.0",
			max:    "1.10.0",
			expect: []string{"1.2.0", "1.10.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterRange(versions, tt.min, tt.max)
			if len(got) != len(tt.expect) {
				t.Errorf("filterRange() = %v, want %v", got, tt.expect)
				return
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("filterRange() = %v, want %v", got, tt.expect)
					return
				}
			}
		})
	}
}

// Integration test with file-based registry and publisher
func TestService_Run_Integration(t *testing.T) {
	t.Parallel(
	// Set up test registry using the shared testutil
	)

	regDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.49.0", "0.50.0"}},
	})

	// Set up destination directory
	destDir := t.TempDir()

	cfg := &config.Config{
		Workflows: []*config.Workflow{
			{
				Name:        "test-sync",
				Description: "Test sync workflow",
				Origin: &config.Registry{
					Type: "file",
					URL:  regDir,
				},
				Destination: &config.Registry{
					Type: "file",
					URL:  destDir,
				},
				Modules: []string{"rules_go"},
				Versions: &config.VersionSelector{
					Type:  "latest",
					Count: 1,
				},
			},
		},
	}

	svc := NewService(cfg)
	result, err := svc.Run(context.Background(), "test-sync", Options{Verbose: true})

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ModulesProcessed != 1 {
		t.Errorf("ModulesProcessed = %d, want 1", result.ModulesProcessed)
	}

	if result.ModulesWritten != 1 {
		t.Errorf("ModulesWritten = %d, want 1", result.ModulesWritten)
	}

	// Verify files were written
	destPub, err := publisher.NewFilePublisher(destDir)
	if err != nil {
		t.Fatalf("NewFilePublisher() error = %v", err)
	}

	exists, err := destPub.Exists(context.Background(), "rules_go/0.50.0/MODULE.bazel")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("MODULE.bazel should exist in destination")
	}
}

func TestService_Run_DryRun(t *testing.T) {
	t.Parallel()
	regDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.49.0", "0.50.0"}},
	})
	destDir := t.TempDir()

	cfg := &config.Config{
		Workflows: []*config.Workflow{
			{
				Name: "test-dry-run",
				Origin: &config.Registry{
					Type: "file",
					URL:  regDir,
				},
				Destination: &config.Registry{
					Type: "file",
					URL:  destDir,
				},
				Modules: []string{"rules_go"},
			},
		},
	}

	svc := NewService(cfg)
	result, err := svc.Run(context.Background(), "test-dry-run", Options{DryRun: true})

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Dry run should report what would be written
	if result.ModulesWritten != 1 {
		t.Errorf("ModulesWritten = %d, want 1 (dry run)", result.ModulesWritten)
	}

	// But nothing should actually be written
	destPub, err := publisher.NewFilePublisher(destDir)
	if err != nil {
		t.Fatalf("NewFilePublisher() error = %v", err)
	}

	exists, err := destPub.Exists(context.Background(), "rules_go/0.50.0/MODULE.bazel")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("MODULE.bazel should NOT exist in destination (dry run)")
	}
}

func TestService_Run_SkipExisting(t *testing.T) {
	t.Parallel()
	regDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.49.0", "0.50.0"}},
	})
	destDir := t.TempDir()

	cfg := &config.Config{
		Workflows: []*config.Workflow{
			{
				Name: "test-skip",
				Origin: &config.Registry{
					Type: "file",
					URL:  regDir,
				},
				Destination: &config.Registry{
					Type: "file",
					URL:  destDir,
				},
				Modules: []string{"rules_go"},
			},
		},
	}

	svc := NewService(cfg)

	// Run once to populate
	result1, err := svc.Run(context.Background(), "test-skip", Options{})
	if err != nil {
		t.Fatalf("First run error = %v", err)
	}
	if result1.ModulesWritten != 1 {
		t.Errorf("First run ModulesWritten = %d, want 1", result1.ModulesWritten)
	}

	// Run again - should skip
	result2, err := svc.Run(context.Background(), "test-skip", Options{})
	if err != nil {
		t.Fatalf("Second run error = %v", err)
	}
	if result2.ModulesWritten != 0 {
		t.Errorf("Second run ModulesWritten = %d, want 0 (already exists)", result2.ModulesWritten)
	}
	if result2.ModulesSkipped != 1 {
		t.Errorf("Second run ModulesSkipped = %d, want 1", result2.ModulesSkipped)
	}
}

func TestService_Run_ModulesOverride(t *testing.T) {
	t.Parallel()
	regDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {Versions: []string{"0.49.0", "0.50.0"}},
		"protobuf": {Versions: []string{"21.7"}},
	})
	destDir := t.TempDir()

	cfg := &config.Config{
		Workflows: []*config.Workflow{
			{
				Name: "test-override",
				Origin: &config.Registry{
					Type: "file",
					URL:  regDir,
				},
				Destination: &config.Registry{
					Type: "file",
					URL:  destDir,
				},
				Modules: []string{"rules_go", "protobuf"},
			},
		},
	}

	svc := NewService(cfg)

	// Override to sync only rules_go (protobuf doesn't have 0.50.0)
	result, err := svc.Run(context.Background(), "test-override", Options{
		Modules: []string{"rules_go"},
	})

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Should only process rules_go
	if result.ModulesProcessed != 1 {
		t.Errorf("ModulesProcessed = %d, want 1", result.ModulesProcessed)
	}
}
