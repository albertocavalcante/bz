package modsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestService_Run_Integration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	regDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
		"rules_go": {
			Versions:       []string{"0.48.0", "0.49.0", "0.50.0"},
			YankedVersions: map[string]string{"0.49.0": "unselected", "0.50.0": "selected"},
		},
	})
	destDir := t.TempDir()

	// Preserve both an old flat output and a configured registry settings file.
	legacyPath := filepath.Join(destDir, "rules_go", "0.50.0", "MODULE.bazel")
	settingsPath := filepath.Join(destDir, "bazel_registry.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{legacyPath: "# legacy copy\n", settingsPath: `{"mirrors":["https://mirror.example/"]}`} {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	workflow := &config.Workflow{
		Name:        "test-sync",
		Origin:      &config.Registry{Type: "file", URL: regDir},
		Destination: &config.Registry{Type: "file", URL: destDir},
		Modules:     []string{"rules_go"},
		Versions:    &config.VersionSelector{Type: "latest", Count: 1},
	}
	svc := NewService(&config.Config{Workflows: []*config.Workflow{workflow}})
	runSync := func(wantWritten, wantSkipped int) {
		t.Helper()
		result, err := svc.Run(ctx, workflow.Name, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Errors) != 0 || result.ModulesProcessed != 1 ||
			result.ModulesWritten != wantWritten || result.ModulesSkipped != wantSkipped {
			t.Fatalf("Run() = %+v, want %d written and %d skipped", result, wantWritten, wantSkipped)
		}
	}
	runSync(1, 0)

	origin := registry.NewFileRegistry(regDir)
	dest := registry.NewFileRegistry(destDir)
	modules, err := dest.ListModules(ctx)
	if err != nil || !slices.Equal(modules, []string{"rules_go"}) {
		t.Fatalf("destination ListModules() = %v, %v", modules, err)
	}
	meta, err := dest.GetMetadata(ctx, "rules_go")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(meta.Versions, []string{"0.50.0"}) || len(meta.YankedVersions) != 1 ||
		meta.YankedVersions["0.50.0"] != "selected" {
		t.Fatalf("unexpected destination metadata: %+v", meta)
	}
	for _, get := range []func(context.Context, string, string) ([]byte, error){dest.GetModuleBazel, dest.GetSource} {
		if _, err := get(ctx, "rules_go", "0.49.0"); !errors.Is(err, registry.ErrVersionNotFound) {
			t.Errorf("read unsynced version: %v", err)
		}
	}

	for _, name := range []string{registry.ModuleBazelPath("rules_go", "0.50.0"), registry.SourcePath("rules_go", "0.50.0")} {
		want, err := os.ReadFile(filepath.Join(regDir, name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(destDir, name))
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("destination %s = %q, %v", name, got, err)
		}
	}
	wantModule, err := origin.GetModuleBazel(ctx, "rules_go", "0.50.0")
	if err != nil {
		t.Fatal(err)
	}
	gotModule, err := dest.GetModuleBazel(ctx, "rules_go", "0.50.0")
	if err != nil || !slices.Equal(gotModule, wantModule) {
		t.Errorf("destination GetModuleBazel() = %q, %v", gotModule, err)
	}
	wantSource, err := origin.GetSource(ctx, "rules_go", "0.50.0")
	if err != nil {
		t.Fatal(err)
	}
	gotSource, err := dest.GetSource(ctx, "rules_go", "0.50.0")
	if err != nil || !slices.Equal(gotSource, wantSource) {
		t.Errorf("destination GetSource() = %q, %v", gotSource, err)
	}
	for name, want := range map[string]string{legacyPath: "# legacy copy\n", settingsPath: `{"mirrors":["https://mirror.example/"]}`} {
		got, err := os.ReadFile(name)
		if err != nil || string(got) != want {
			t.Errorf("contents of %s = %q, %v", name, got, err)
		}
	}

	// An incremental backfill keeps previously published versions and local metadata.
	metadataPath := filepath.Join(destDir, registry.MetadataPath("rules_go"))
	priorMetadata := `{"versions":["0.50.0"],"yanked_versions":{"0.50.0":"selected"},"local_note":"keep"}`
	if err := os.WriteFile(metadataPath, []byte(priorMetadata), 0o644); err != nil {
		t.Fatal(err)
	}
	workflow.Versions = &config.VersionSelector{Type: "range", MaxVer: "0.48.0"}
	runSync(1, 0)
	meta, err = dest.GetMetadata(ctx, "rules_go")
	if err != nil || !slices.Equal(meta.Versions, []string{"0.48.0", "0.50.0"}) || meta.YankedVersions["0.50.0"] != "selected" {
		t.Fatalf("destination metadata after backfill = %+v, %v", meta, err)
	}
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || string(fields["local_note"]) != `"keep"` {
		t.Errorf("destination metadata extensions = %s, %v", data, err)
	}
	runSync(0, 1)
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
	if result.ModulesWritten != 0 || result.VersionsPlanned != 1 {
		t.Errorf("written = %d, planned = %d; want 0, 1", result.ModulesWritten, result.VersionsPlanned)
	}

	// But nothing should actually be written
	destPub, err := publisher.NewFileRegistryPublisher(destDir)
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

func TestService_Run_RewritesOnlySourceURLs(t *testing.T) {
	t.Parallel()
	const name = "example"
	const version = "1.0.0"
	const patch = "--- a/message.txt\n+++ b/message.txt\n@@ -1 +1 @@\n-before\n+after\n"
	checksum := sha256.Sum256([]byte(patch))
	integrity := "sha256-" + base64.StdEncoding.EncodeToString(checksum[:])
	archive := fmt.Sprintf(`{
  "url": "https://upstream.example/source.tar.gz",
  "mirror_urls": ["https://backup.example/source.tar.gz", "https://upstream.example/backup.tar.gz"],
  "urls": ["https://upstream.example/legacy.tar.gz"],
  "archive": "https://upstream.example/other.tar.gz",
  "archive_type": "tar.gz",
  "type": "archive",
  "patches": {"second.patch":%[1]q, "first.patch":%[1]q},
  "extra": { "url": "https://upstream.example/info", "number": 9007199254740993, "raw": "\u0061" }
}`, integrity)
	git := fmt.Sprintf(`{"url":"https://upstream.example/extension","mirror_urls":{"extension":true},"remote":"https://upstream.example/project.git","commit":"abc","tag":"v1","type":"git_repository","patches":{"fix.patch":%q}}`, integrity)
	const local = `{"urls":["https://upstream.example/extension"],"archive":"https://upstream.example/other","path":"relative/dependency","type":"local_path","extension":true}`
	tests := []struct {
		name       string
		descriptor string
		want       string
	}{
		{
			name:       "archive",
			descriptor: archive,
			want: strings.NewReplacer(
				"https://upstream.example/source.tar.gz", "https://mirror.example/source.tar.gz",
				"https://upstream.example/backup.tar.gz", "https://mirror.example/backup.tar.gz",
				"https://upstream.example/legacy.tar.gz", "https://mirror.example/legacy.tar.gz",
				"https://upstream.example/other.tar.gz", "https://mirror.example/other.tar.gz",
			).Replace(archive),
		},
		{name: "git", descriptor: git, want: git},
		{name: "local", descriptor: local, want: local},
	}

	for _, kind := range []string{"file", "http"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					originDir := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{
						name: {Versions: []string{version}},
					})
					sourcePath := filepath.FromSlash(registry.SourcePath(name, version))
					if err := os.WriteFile(filepath.Join(originDir, sourcePath), []byte(tt.descriptor), 0o644); err != nil {
						t.Fatal(err)
					}
					patchDir := filepath.Join(originDir, filepath.Dir(sourcePath), "patches")
					if err := os.MkdirAll(patchDir, 0o755); err != nil {
						t.Fatal(err)
					}
					for _, patchName := range []string{"first.patch", "second.patch", "fix.patch"} {
						if err := os.WriteFile(filepath.Join(patchDir, patchName), []byte(patch), 0o644); err != nil {
							t.Fatal(err)
						}
					}
					origin := &config.Registry{Type: kind, URL: originDir}
					if kind == "http" {
						server := httptest.NewServer(http.FileServer(http.Dir(originDir)))
						t.Cleanup(server.Close)
						origin.URL = server.URL
					}

					dest := t.TempDir()
					workflow := &config.Workflow{
						Name:        "rewrite",
						Origin:      origin,
						Destination: &config.Registry{Type: "file", URL: dest},
						Modules:     []string{name},
						Transformations: []*config.Transform{
							{Type: "skip_yanked"},
							{Type: "rewrite_urls", Pattern: `^https://upstream\.example/`, Replacement: "https://mirror.example/"},
						},
					}
					result, err := NewService(&config.Config{Workflows: []*config.Workflow{workflow}}).
						Run(t.Context(), workflow.Name, Options{})
					if err != nil || len(result.Errors) != 0 || result.ModulesWritten != 1 {
						t.Fatalf("Run() = %+v, %v", result, err)
					}

					if tt.name != "local" {
						patchName := "first.patch"
						if tt.name == "git" {
							patchName = "fix.patch"
						}
						missing := filepath.Join(dest, registry.ModulesDir, name, version, "patches", patchName)
						if err := os.Remove(missing); err != nil {
							t.Fatal(err)
						}
						preview, err := NewService(&config.Config{Workflows: []*config.Workflow{workflow}}).Run(t.Context(), workflow.Name, Options{DryRun: true})
						if err != nil || len(preview.Errors) != 0 || preview.VersionsPlanned != 1 || preview.ModulesWritten != 0 {
							t.Fatalf("repair preview = %+v, %v", preview, err)
						}
						if _, err := os.Stat(missing); !os.IsNotExist(err) {
							t.Fatalf("preview changed missing patch: %v", err)
						}
					}

					got, err := os.ReadFile(filepath.Join(dest, registry.ModulesDir, name, version, registry.SourceFile))
					if err != nil {
						t.Fatal(err)
					}
					if string(got) != tt.want {
						t.Errorf("destination source.json = %s\nwant %s", got, tt.want)
					}
				})
			}
		})
	}
}

func TestFileMirrorConcurrentCatalog(t *testing.T) {
	origin := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{"example": {Versions: []string{"1.0.0", "2.0.0"}}})
	dest := t.TempDir()
	workflows := make([]*config.Workflow, 0, 2)
	for _, version := range []string{"1.0.0", "2.0.0"} {
		workflows = append(workflows, &config.Workflow{Name: version, Origin: &config.Registry{Type: "file", URL: origin}, Destination: &config.Registry{Type: "file", URL: dest}, Modules: []string{"example"}, Versions: &config.VersionSelector{Type: "range", MinVer: version, MaxVer: version}})
	}
	svc := NewService(&config.Config{Workflows: workflows})
	errs := make(chan error, len(workflows))
	for _, workflow := range workflows {
		go func(name string) {
			result, err := svc.Run(context.Background(), name, Options{})
			if err == nil && len(result.Errors) > 0 {
				err = result.Errors[0]
			}
			errs <- err
		}(workflow.Name)
	}
	for range workflows {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	meta, err := registry.NewFileRegistry(dest).GetMetadata(context.Background(), "example")
	if err != nil || !slices.Equal(meta.Versions, []string{"1.0.0", "2.0.0"}) {
		t.Fatalf("catalog = %+v, %v", meta, err)
	}
}

func TestFileMirrorYankReconciliation(t *testing.T) {
	origin := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{"example": {Versions: []string{"1.0.0", "2.0.0"}, YankedVersions: map[string]string{"1.0.0": "old"}}})
	dest := t.TempDir()
	workflow := &config.Workflow{Name: "mirror", Origin: &config.Registry{Type: "file", URL: origin}, Destination: &config.Registry{Type: "file", URL: dest}, Modules: []string{"example"}, Versions: &config.VersionSelector{Type: "all"}}
	svc := NewService(&config.Config{Workflows: []*config.Workflow{workflow}})
	run := func() {
		t.Helper()
		result, err := svc.Run(context.Background(), "mirror", Options{})
		if err != nil || len(result.Errors) != 0 {
			t.Fatalf("sync = %+v, %v", result, err)
		}
	}
	run()
	// A destination-only complete version keeps its yank; stale yank keys are removed.
	localVersion := filepath.Join(dest, "modules", "example", "0.5.0")
	if err := os.MkdirAll(localVersion, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"MODULE.bazel", "source.json"} {
		data, err := os.ReadFile(filepath.Join(dest, "modules", "example", "1.0.0", name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "MODULE.bazel" {
			data = []byte(`module(name = "example", version = "0.5.0")`)
		}
		if err := os.WriteFile(filepath.Join(localVersion, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	localMeta := filepath.Join(dest, registry.MetadataPath("example"))
	if err := os.WriteFile(localMeta, []byte(`{"versions":["1.0.0","2.0.0","0.5.0"],"yanked_versions":{"1.0.0":"old","0.5.0":"local","9.0.0":"stale"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(origin, registry.MetadataPath("example"))
	if err := os.WriteFile(path, []byte(`{"versions":["1.0.0","2.0.0"],"yanked_versions":{"2.0.0":"new"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	workflow.Transformations = []*config.Transform{{Type: "skip_yanked"}}
	run()
	meta, err := registry.NewFileRegistry(dest).GetMetadata(context.Background(), "example")
	if err != nil || len(meta.YankedVersions) != 2 || meta.YankedVersions["2.0.0"] != "new" || meta.YankedVersions["0.5.0"] != "local" {
		t.Fatalf("yanks = %+v, %v", meta, err)
	}
}

func TestFileMirrorRejectsInvalidFreshCoreAndRecovers(t *testing.T) {
	for _, tc := range []struct{ file, bad string }{{"MODULE.bazel", "module("}, {"MODULE.bazel", ""}, {"MODULE.bazel", `module(name = "wrong", version = "1.0.0")`}, {"MODULE.bazel", `module(name = "example", version = "2.0.0")`}, {"source.json", "[]"}, {"source.json", "null"}} {
		t.Run(tc.file+tc.bad, func(t *testing.T) {
			origin := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{"example": {Versions: []string{"1.0.0"}}})
			dest := t.TempDir()
			file := filepath.Join(origin, "modules", "example", "1.0.0", tc.file)
			good, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(tc.bad), 0o644); err != nil {
				t.Fatal(err)
			}
			workflow := &config.Workflow{Name: "mirror", Origin: &config.Registry{Type: "file", URL: origin}, Destination: &config.Registry{Type: "file", URL: dest}, Modules: []string{"example"}}
			svc := NewService(&config.Config{Workflows: []*config.Workflow{workflow}})
			result, err := svc.Run(context.Background(), "mirror", Options{})
			if err != nil || len(result.Errors) == 0 {
				t.Fatalf("invalid sync = %+v, %v", result, err)
			}
			if _, err := os.Stat(filepath.Join(dest, registry.MetadataPath("example"))); !os.IsNotExist(err) {
				t.Fatalf("invalid catalog exists: %v", err)
			}
			if err := os.WriteFile(file, good, 0o644); err != nil {
				t.Fatal(err)
			}
			result, err = svc.Run(context.Background(), "mirror", Options{})
			if err != nil || len(result.Errors) != 0 {
				t.Fatalf("recovery = %+v, %v", result, err)
			}
			if tc.file == "source.json" {
				destinationFile := filepath.Join(dest, registry.SourcePath("example", "1.0.0"))
				if err := os.WriteFile(destinationFile, []byte(tc.bad), 0o644); err != nil {
					t.Fatal(err)
				}
				result, err = svc.Run(context.Background(), "mirror", Options{})
				if err != nil || len(result.Errors) != 0 {
					t.Fatalf("destination repair = %+v, %v", result, err)
				}
				got, err := os.ReadFile(destinationFile)
				if err != nil || !bytes.Equal(got, good) {
					t.Fatalf("repaired source = %q, %v; want %q", got, err, good)
				}
			}
		})
	}
}

func TestFileMirrorReconcilesWhenDefaultSelectorFindsNoVersion(t *testing.T) {
	origin := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{"example": {Versions: []string{"1.0.0"}}})
	dest := t.TempDir()
	workflow := &config.Workflow{Name: "mirror", Origin: &config.Registry{Type: "file", URL: origin}, Destination: &config.Registry{Type: "file", URL: dest}, Modules: []string{"example"}}
	svc := NewService(&config.Config{Workflows: []*config.Workflow{workflow}})
	run := func() {
		t.Helper()
		result, err := svc.Run(context.Background(), "mirror", Options{})
		if err != nil || len(result.Errors) != 0 {
			t.Fatalf("sync = %+v, %v", result, err)
		}
	}
	run()
	metadata := filepath.Join(origin, registry.MetadataPath("example"))
	if err := os.WriteFile(metadata, []byte(`{"versions":["1.0.0"],"yanked_versions":{"1.0.0":"withdrawn"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run()
	got, err := registry.NewFileRegistry(dest).GetMetadata(context.Background(), "example")
	if err != nil || got.YankedVersions["1.0.0"] != "withdrawn" {
		t.Fatalf("metadata = %+v, %v", got, err)
	}
	// A first sync with no selected version must not create an empty catalog.
	workflow.Destination.URL = t.TempDir()
	run()
	if _, err := os.Stat(filepath.Join(workflow.Destination.URL, registry.MetadataPath("example"))); !os.IsNotExist(err) {
		t.Fatalf("unexpected catalog: %v", err)
	}
}

func TestFileMirrorRejectsOversizedOrigin(t *testing.T) {
	origin := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{"example": {Versions: []string{"1.0.0"}}})
	file, err := os.OpenFile(filepath.Join(origin, registry.ModuleBazelPath("example", "1.0.0")), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(registry.MaxFileBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	workflow := &config.Workflow{Name: "mirror", Origin: &config.Registry{Type: "file", URL: origin}, Destination: &config.Registry{Type: "file", URL: dest}, Modules: []string{"example"}}
	result, err := NewService(&config.Config{Workflows: []*config.Workflow{workflow}}).Run(context.Background(), "mirror", Options{})
	if err != nil || len(result.Errors) == 0 {
		t.Fatalf("oversized sync = %+v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(dest, registry.MetadataPath("example"))); !os.IsNotExist(err) {
		t.Fatalf("unexpected catalog: %v", err)
	}
}

func TestValidModuleIdentity(t *testing.T) {
	for _, tc := range []struct {
		declared, requested string
		want                bool
	}{{"1.0+upstream", "1.0+mirror", true}, {"01.0", "1.0", false}, {"1.0", "1.0.0", false}, {"2025.foo", "2025.foo", true}, {"1.0+", "1.0", false}, {"1.0+bad/part", "1.0", false}} {
		content := []byte(`module(name = "example", version = "` + tc.declared + `")`)
		if got := validModule(content, "example", tc.requested); got != tc.want {
			t.Errorf("version %q for %q = %v", tc.declared, tc.requested, got)
		}
	}
}

func TestValidModuleDynamicIdentity(t *testing.T) {
	for _, content := range []string{`module(name = "example", version = "1." + "0")`, `module(**{"name": "example", "version": "1.0"})`, `module(**args)`} {
		if !validModule([]byte(content), "example", "1.0") {
			t.Errorf("dynamic declaration rejected: %s", content)
		}
	}
}

func TestLiteralExpansionMismatch(t *testing.T) {
	if validModule([]byte(`module(**{"name":"wrong", "version":"1.0"})`), "example", "1.0") {
		t.Fatal("accepted wrong name")
	}
	if validModule([]byte(`module(**{"name":"example", "version":"1.0+"})`), "example", "1.0") {
		t.Fatal("accepted invalid version")
	}
}

func TestFileMirrorNormalizesYankedKeys(t *testing.T) {
	origin := testutil.SetupTestRegistry(t, map[string]testutil.TestModule{"example": {Versions: []string{"1.0"}}})
	dest := t.TempDir()
	workflow := &config.Workflow{Name: "mirror", Origin: &config.Registry{Type: "file", URL: origin}, Destination: &config.Registry{Type: "file", URL: dest}, Modules: []string{"example"}}
	svc := NewService(&config.Config{Workflows: []*config.Workflow{workflow}})
	run := func() {
		t.Helper()
		result, err := svc.Run(context.Background(), "mirror", Options{})
		if err != nil || len(result.Errors) > 0 {
			t.Fatalf("sync = %+v, %v", result, err)
		}
	}
	run()
	metaPath := filepath.Join(origin, registry.MetadataPath("example"))
	if err := os.WriteFile(metaPath, []byte(`{"versions":["1.0"],"yanked_versions":{"1.0+origin":"withdrawn"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run()
	meta, err := registry.NewFileRegistry(dest).GetMetadata(context.Background(), "example")
	if err != nil || meta.YankedVersions["1.0"] != "withdrawn" {
		t.Fatalf("metadata = %+v, %v", meta, err)
	}
	// Explicit all plus skip_yanked uses the same normalized lookup.
	workflow.Destination.URL = t.TempDir()
	workflow.Versions = &config.VersionSelector{Type: "all"}
	workflow.Transformations = []*config.Transform{{Type: "skip_yanked"}}
	run()
	if _, err := os.Stat(filepath.Join(workflow.Destination.URL, registry.MetadataPath("example"))); !os.IsNotExist(err) {
		t.Fatalf("yanked version was published: %v", err)
	}
}

func TestReconcileYanksDeduplicatesNormalizedVersions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		origin *registry.Metadata
		prior  map[string]string
	}{
		{name: "origin", origin: &registry.Metadata{Versions: []string{"1.0", "1.0+build"}, YankedVersions: map[string]string{"1.0+build": "withdrawn"}}},
		{name: "destination only", origin: &registry.Metadata{}, prior: map[string]string{"1.0+build": "withdrawn"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := reconcileYanks(tc.origin, tc.prior, []string{"1.0", "1.0+build"})
			if len(got) != 1 || got["1.0"] != "withdrawn" {
				t.Fatalf("yanks = %v", got)
			}
		})
	}
}
