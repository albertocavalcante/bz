package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func TestSyncLatest(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected int
		wantErr  string
	}{
		{
			name:     "default count",
			code:     `sync.latest()`,
			expected: 1,
		},
		{
			name:     "explicit count",
			code:     `sync.latest(count = 3)`,
			expected: 3,
		},
		{
			name:     "positional count",
			code:     `sync.latest(5)`,
			expected: 5,
		},
		{
			name:    "invalid count zero",
			code:    `sync.latest(count = 0)`,
			wantErr: "count must be at least 1",
		},
		{
			name:    "invalid count negative",
			code:    `sync.latest(count = -1)`,
			wantErr: "count must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
			globals := starlark.StringDict{"sync": syncModule}

			result, err := starlark.Eval(thread, "test.star", tt.code, globals)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			selector, ok := result.(*VersionSelector)
			require.True(t, ok, "expected VersionSelector, got %T", result)
			assert.Equal(t, VersionSelectorLatest, selector.Kind)
			assert.Equal(t, tt.expected, selector.Count)
		})
	}
}

func TestSyncAll(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
	globals := starlark.StringDict{"sync": syncModule}

	result, err := starlark.Eval(thread, "test.star", `sync.all()`, globals)
	require.NoError(t, err)

	selector, ok := result.(*VersionSelector)
	require.True(t, ok, "expected VersionSelector, got %T", result)
	assert.Equal(t, VersionSelectorAll, selector.Kind)
}

func TestSyncSince(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
		wantErr  string
	}{
		{
			name:     "valid date",
			code:     `sync.since(date = "2024-01-01")`,
			expected: "2024-01-01",
		},
		{
			name:     "positional date",
			code:     `sync.since("2023-06-15")`,
			expected: "2023-06-15",
		},
		{
			name:    "missing date",
			code:    `sync.since()`,
			wantErr: "missing required argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
			globals := starlark.StringDict{"sync": syncModule}

			result, err := starlark.Eval(thread, "test.star", tt.code, globals)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			selector, ok := result.(*VersionSelector)
			require.True(t, ok, "expected VersionSelector, got %T", result)
			assert.Equal(t, VersionSelectorSince, selector.Kind)
			assert.Equal(t, tt.expected, selector.Since)
		})
	}
}

func TestSyncRange(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		expectedMin string
		expectedMax string
		wantErr     string
	}{
		{
			name:        "both bounds",
			code:        `sync.range(min = "1.0.0", max = "2.0.0")`,
			expectedMin: "1.0.0",
			expectedMax: "2.0.0",
		},
		{
			name:        "min only",
			code:        `sync.range(min = "1.0.0")`,
			expectedMin: "1.0.0",
			expectedMax: "",
		},
		{
			name:        "max only",
			code:        `sync.range(max = "2.0.0")`,
			expectedMin: "",
			expectedMax: "2.0.0",
		},
		{
			name:    "no bounds",
			code:    `sync.range()`,
			wantErr: "at least one of 'min' or 'max' must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
			globals := starlark.StringDict{"sync": syncModule}

			result, err := starlark.Eval(thread, "test.star", tt.code, globals)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			selector, ok := result.(*VersionSelector)
			require.True(t, ok, "expected VersionSelector, got %T", result)
			assert.Equal(t, VersionSelectorRange, selector.Kind)
			assert.Equal(t, tt.expectedMin, selector.MinVer)
			assert.Equal(t, tt.expectedMax, selector.MaxVer)
		})
	}
}

func TestSyncTransformations(t *testing.T) {
	t.Run("rewrite_source_urls", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		globals := starlark.StringDict{"sync": syncModule}

		code := `sync.rewrite_source_urls(pattern = "https://github.com/(.+)/archive/(.+)", replacement = "https://internal/$1/$2")`
		result, err := starlark.Eval(thread, "test.star", code, globals)
		require.NoError(t, err)

		transform, ok := result.(*TransformValue)
		require.True(t, ok, "expected TransformValue, got %T", result)
		assert.Equal(t, TransformRewriteURLs, transform.Kind)
		assert.Equal(t, "https://github.com/(.+)/archive/(.+)", transform.Pattern)
		assert.Equal(t, "https://internal/$1/$2", transform.Replacement)
	})

	t.Run("rewrite_source_urls missing args", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		globals := starlark.StringDict{"sync": syncModule}

		_, err := starlark.Eval(thread, "test.star", `sync.rewrite_source_urls(pattern = "test")`, globals)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required argument")
	})

	t.Run("skip_yanked", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		globals := starlark.StringDict{"sync": syncModule}

		result, err := starlark.Eval(thread, "test.star", `sync.skip_yanked()`, globals)
		require.NoError(t, err)

		transform, ok := result.(*TransformValue)
		require.True(t, ok, "expected TransformValue, got %T", result)
		assert.Equal(t, TransformSkipYanked, transform.Kind)
	})

	t.Run("include_patches default", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		globals := starlark.StringDict{"sync": syncModule}

		result, err := starlark.Eval(thread, "test.star", `sync.include_patches()`, globals)
		require.NoError(t, err)

		transform, ok := result.(*TransformValue)
		require.True(t, ok, "expected TransformValue, got %T", result)
		assert.Equal(t, TransformIncludePatches, transform.Kind)
		assert.True(t, transform.Enabled)
	})

	t.Run("include_patches disabled", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		globals := starlark.StringDict{"sync": syncModule}

		result, err := starlark.Eval(thread, "test.star", `sync.include_patches(enabled = False)`, globals)
		require.NoError(t, err)

		transform, ok := result.(*TransformValue)
		require.True(t, ok, "expected TransformValue, got %T", result)
		assert.Equal(t, TransformIncludePatches, transform.Kind)
		assert.False(t, transform.Enabled)
	})
}

func TestSyncWorkflow(t *testing.T) {
	t.Run("basic workflow", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetWorkflowRegistry(thread)

		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
		globals := starlark.StringDict{
			"sync":     syncModule,
			"registry": registryModule,
		}

		code := `
bcr = registry.http("https://bcr.bazel.build")
internal = registry.file("/path/to/internal")
sync.workflow(
    name = "mirror",
    description = "Mirror modules",
    origin = bcr,
    destination = internal,
    modules = ["rules_go", "rules_python"],
)
`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.NoError(t, err)

		workflows := GetWorkflows(thread)
		require.Len(t, workflows, 1)

		w := workflows[0]
		assert.Equal(t, "mirror", w.Name)
		assert.Equal(t, "Mirror modules", w.Description)
		assert.Equal(t, RegistryTypeHTTP, w.Origin.Kind)
		assert.Equal(t, "https://bcr.bazel.build", w.Origin.URL)
		assert.Equal(t, RegistryTypeFile, w.Destination.Kind)
		assert.Equal(t, "/path/to/internal", w.Destination.URL)
		assert.Equal(t, []string{"rules_go", "rules_python"}, w.Modules)

		// Default version selector
		assert.NotNil(t, w.Versions)
		assert.Equal(t, VersionSelectorLatest, w.Versions.Kind)
		assert.Equal(t, 1, w.Versions.Count)
	})

	t.Run("workflow with all options", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetWorkflowRegistry(thread)

		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
		globals := starlark.StringDict{
			"sync":     syncModule,
			"registry": registryModule,
		}

		code := `
bcr = registry.http("https://bcr.bazel.build")
internal = registry.file("/path/to/internal")
sync.workflow(
    name = "mirror-essential",
    description = "Mirror essential modules",
    origin = bcr,
    destination = internal,
    modules = ["rules_go", "bazel_skylib"],
    versions = sync.latest(count = 3),
    transformations = [
        sync.rewrite_source_urls(
            pattern = "https://github.com/(.+)/archive/(.+)",
            replacement = "https://internal/$1/$2",
        ),
        sync.skip_yanked(),
    ],
)
`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.NoError(t, err)

		workflows := GetWorkflows(thread)
		require.Len(t, workflows, 1)

		w := workflows[0]
		assert.Equal(t, "mirror-essential", w.Name)
		assert.Equal(t, VersionSelectorLatest, w.Versions.Kind)
		assert.Equal(t, 3, w.Versions.Count)
		assert.Len(t, w.Transformations, 2)
		assert.Equal(t, TransformRewriteURLs, w.Transformations[0].Kind)
		assert.Equal(t, TransformSkipYanked, w.Transformations[1].Kind)
	})

	t.Run("multiple workflows", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetWorkflowRegistry(thread)

		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
		globals := starlark.StringDict{
			"sync":     syncModule,
			"registry": registryModule,
		}

		code := `
bcr = registry.http("https://bcr.bazel.build")
internal = registry.file("/path/to/internal")

sync.workflow(
    name = "workflow1",
    origin = bcr,
    destination = internal,
    modules = ["rules_go"],
)

sync.workflow(
    name = "workflow2",
    origin = bcr,
    destination = internal,
    modules = ["rules_python"],
)
`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.NoError(t, err)

		workflows := GetWorkflows(thread)
		require.Len(t, workflows, 2)
		assert.Equal(t, "workflow1", workflows[0].Name)
		assert.Equal(t, "workflow2", workflows[1].Name)
	})

	t.Run("workflow without registry initialized", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		// NOT calling SetWorkflowRegistry

		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
		globals := starlark.StringDict{
			"sync":     syncModule,
			"registry": registryModule,
		}

		code := `
bcr = registry.http("https://bcr.bazel.build")
internal = registry.file("/path/to/internal")
sync.workflow(
    name = "mirror",
    origin = bcr,
    destination = internal,
    modules = ["rules_go"],
)
`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow registry not initialized")
	})

	t.Run("workflow with invalid origin", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetWorkflowRegistry(thread)

		syncModule := starlarkstruct.FromStringDict(starlark.String("sync"), SyncModule())
		registryModule := starlarkstruct.FromStringDict(starlark.String("registry"), RegistryModule())
		globals := starlark.StringDict{
			"sync":     syncModule,
			"registry": registryModule,
		}

		code := `
internal = registry.file("/path/to/internal")
sync.workflow(
    name = "mirror",
    origin = "not a registry",
    destination = internal,
    modules = ["rules_go"],
)
`
		_, err := starlark.ExecFile(thread, "test.star", code, globals)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "origin must be a registry value")
	})
}

func TestWorkflowRegistry(t *testing.T) {
	t.Run("set and get workflows", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		SetWorkflowRegistry(thread)

		// Initially empty
		workflows := GetWorkflows(thread)
		assert.Empty(t, workflows)
	})

	t.Run("get workflows when not set", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		workflows := GetWorkflows(thread)
		assert.Nil(t, workflows)
	})
}

func TestVersionSelectorString(t *testing.T) {
	tests := []struct {
		name     string
		selector *VersionSelector
		expected string
	}{
		{
			name: "latest",
			selector: &VersionSelector{
				Kind:  VersionSelectorLatest,
				Count: 3,
			},
			expected: "sync.latest(count=3)",
		},
		{
			name: "all",
			selector: &VersionSelector{
				Kind: VersionSelectorAll,
			},
			expected: "sync.all()",
		},
		{
			name: "since",
			selector: &VersionSelector{
				Kind:  VersionSelectorSince,
				Since: "2024-01-01",
			},
			expected: `sync.since(date="2024-01-01")`,
		},
		{
			name: "range",
			selector: &VersionSelector{
				Kind:   VersionSelectorRange,
				MinVer: "1.0.0",
				MaxVer: "2.0.0",
			},
			expected: `sync.range(min="1.0.0", max="2.0.0")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.selector.String())
		})
	}
}

func TestTransformValueString(t *testing.T) {
	tests := []struct {
		name      string
		transform *TransformValue
		expected  string
	}{
		{
			name: "rewrite_urls",
			transform: &TransformValue{
				Kind:        TransformRewriteURLs,
				Pattern:     "pattern",
				Replacement: "replacement",
			},
			expected: `sync.rewrite_source_urls(pattern="pattern", replacement="replacement")`,
		},
		{
			name: "skip_yanked",
			transform: &TransformValue{
				Kind: TransformSkipYanked,
			},
			expected: "sync.skip_yanked()",
		},
		{
			name: "include_patches enabled",
			transform: &TransformValue{
				Kind:    TransformIncludePatches,
				Enabled: true,
			},
			expected: "sync.include_patches(enabled=true)",
		},
		{
			name: "include_patches disabled",
			transform: &TransformValue{
				Kind:    TransformIncludePatches,
				Enabled: false,
			},
			expected: "sync.include_patches(enabled=false)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.transform.String())
		})
	}
}

func TestWorkflowValueString(t *testing.T) {
	w := &WorkflowValue{
		Name: "test-workflow",
	}
	assert.Equal(t, `sync.workflow(name="test-workflow")`, w.String())
}

func TestSyncModule(t *testing.T) {
	module := SyncModule()

	assert.Contains(t, module, "workflow")
	assert.Contains(t, module, "latest")
	assert.Contains(t, module, "all")
	assert.Contains(t, module, "since")
	assert.Contains(t, module, "range")
	assert.Contains(t, module, "rewrite_source_urls")
	assert.Contains(t, module, "skip_yanked")
	assert.Contains(t, module, "include_patches")
}
