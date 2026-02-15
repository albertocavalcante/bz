package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/albertocavalcante/bz/internal/config/modules"
)

func TestLoaderLoad(t *testing.T) {
	t.Parallel()
	t.Run("complete config file", func(t *testing.T) {
		t.Parallel(
		// Create a temporary config file
		)

		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bz.star")

		configContent := `
bcr = registry.http("https://bcr.bazel.build")
local = registry.file("/tmp/registry")

sync.workflow(
    name = "test",
    origin = bcr,
    destination = local,
    modules = ["rules_go"],
)

config.defaults(
    registry = bcr,
)
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader()
		cfg, err := loader.Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Check workflows
		require.Len(t, cfg.Workflows, 1)
		wf := cfg.Workflows[0]
		assert.Equal(t, "test", wf.Name)
		assert.Equal(t, "http", wf.Origin.Type)
		assert.Equal(t, "https://bcr.bazel.build", wf.Origin.URL)
		assert.Equal(t, "file", wf.Destination.Type)
		assert.Equal(t, "/tmp/registry", wf.Destination.URL)
		assert.Equal(t, []string{"rules_go"}, wf.Modules)

		// Check defaults
		require.NotNil(t, cfg.Defaults)
		require.NotNil(t, cfg.Defaults.Registry)
		assert.Equal(t, "http", cfg.Defaults.Registry.Type)
		assert.Equal(t, "https://bcr.bazel.build", cfg.Defaults.Registry.URL)
	})

	t.Run("workflow with all options", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bz.star")

		configContent := `
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
        sync.include_patches(enabled = True),
    ],
)
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader()
		cfg, err := loader.Load(configPath)
		require.NoError(t, err)
		require.Len(t, cfg.Workflows, 1)

		wf := cfg.Workflows[0]
		assert.Equal(t, "mirror-essential", wf.Name)
		assert.Equal(t, "Mirror essential modules", wf.Description)
		assert.Equal(t, []string{"rules_go", "bazel_skylib"}, wf.Modules)

		// Check versions
		require.NotNil(t, wf.Versions)
		assert.Equal(t, "latest", wf.Versions.Type)
		assert.Equal(t, 3, wf.Versions.Count)

		// Check transformations
		require.Len(t, wf.Transformations, 3)
		assert.Equal(t, "rewrite_urls", wf.Transformations[0].Type)
		assert.Equal(t, "https://github.com/(.+)/archive/(.+)", wf.Transformations[0].Pattern)
		assert.Equal(t, "https://internal/$1/$2", wf.Transformations[0].Replacement)
		assert.Equal(t, "skip_yanked", wf.Transformations[1].Type)
		assert.Equal(t, "include_patches", wf.Transformations[2].Type)
		assert.True(t, wf.Transformations[2].Enabled)
	})

	t.Run("multiple workflows", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bz.star")

		configContent := `
bcr = registry.http("https://bcr.bazel.build")
local = registry.file("/tmp/registry")

sync.workflow(
    name = "workflow1",
    origin = bcr,
    destination = local,
    modules = ["rules_go"],
)

sync.workflow(
    name = "workflow2",
    origin = bcr,
    destination = local,
    modules = ["rules_python"],
)
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader()
		cfg, err := loader.Load(configPath)
		require.NoError(t, err)
		require.Len(t, cfg.Workflows, 2)
		assert.Equal(t, "workflow1", cfg.Workflows[0].Name)
		assert.Equal(t, "workflow2", cfg.Workflows[1].Name)
	})

	t.Run("config with all registry types", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bz.star")

		configContent := `
http_reg = registry.http("https://bcr.bazel.build")
file_reg = registry.file("/tmp/registry")
git_reg = registry.git(url = "https://github.com/example/registry.git", branch = "main")
http_put_reg = registry.http_put(
    url = "https://registry.example.com",
    auth = auth.bearer_token(env = "REGISTRY_TOKEN"),
)

sync.workflow(
    name = "test",
    origin = http_reg,
    destination = file_reg,
    modules = ["test"],
)

config.defaults(
    registry = http_reg,
    cache_dir = "~/.cache/bz",
    fallback_registries = [file_reg, git_reg],
)
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader()
		cfg, err := loader.Load(configPath)
		require.NoError(t, err)

		require.NotNil(t, cfg.Defaults)
		assert.Equal(t, "http", cfg.Defaults.Registry.Type)
		assert.Equal(t, "~/.cache/bz", cfg.Defaults.CacheDir)
		require.Len(t, cfg.Defaults.FallbackRegistries, 2)
		assert.Equal(t, "file", cfg.Defaults.FallbackRegistries[0].Type)
		assert.Equal(t, "git", cfg.Defaults.FallbackRegistries[1].Type)
		assert.Equal(t, "main", cfg.Defaults.FallbackRegistries[1].Branch)
	})

	t.Run("git registry with ref", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bz.star")

		configContent := `
git_reg = registry.git(url = "https://github.com/example/registry.git", ref = "v1.0.0")
local = registry.file("/tmp/registry")

sync.workflow(
    name = "test",
    origin = git_reg,
    destination = local,
    modules = ["test"],
)
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader()
		cfg, err := loader.Load(configPath)
		require.NoError(t, err)
		require.Len(t, cfg.Workflows, 1)

		origin := cfg.Workflows[0].Origin
		assert.Equal(t, "git", origin.Type)
		assert.Equal(t, "v1.0.0", origin.Ref)
		assert.Empty(t, origin.Branch)
	})

	t.Run("http_put with auth", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bz.star")

		configContent := `
bcr = registry.http("https://bcr.bazel.build")
dest = registry.http_put(
    url = "https://registry.example.com",
    auth = auth.basic(username = "user", password = "pass"),
)

sync.workflow(
    name = "test",
    origin = bcr,
    destination = dest,
    modules = ["test"],
)
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader()
		cfg, err := loader.Load(configPath)
		require.NoError(t, err)
		require.Len(t, cfg.Workflows, 1)

		dest := cfg.Workflows[0].Destination
		assert.Equal(t, "http_put", dest.Type)
		require.NotNil(t, dest.Auth)
		assert.Equal(t, "basic", dest.Auth.Type)
		assert.Equal(t, "user", dest.Auth.Username)
		assert.Equal(t, "pass", dest.Auth.Password)
	})

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()
		loader := NewLoader()
		_, err := loader.Load("/nonexistent/path/bz.star")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config file not found")
	})

	t.Run("invalid starlark syntax", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bz.star")

		configContent := `
this is not valid starlark
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader()
		_, err = loader.Load(configPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "executing config file")
	})

	t.Run("version selectors", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name         string
			versionCode  string
			expectedType string
			checkFn      func(t *testing.T, v *VersionSelector)
		}{
			{
				name:         "all",
				versionCode:  "versions = sync.all(),",
				expectedType: "all",
				checkFn:      func(t *testing.T, v *VersionSelector) {},
			},
			{
				name:         "since",
				versionCode:  `versions = sync.since(date = "2024-01-01"),`,
				expectedType: "since",
				checkFn: func(t *testing.T, v *VersionSelector) {
					assert.Equal(t, "2024-01-01", v.Since)
				},
			},
			{
				name:         "range",
				versionCode:  `versions = sync.range(min = "1.0.0", max = "2.0.0"),`,
				expectedType: "range",
				checkFn: func(t *testing.T, v *VersionSelector) {
					assert.Equal(t, "1.0.0", v.MinVer)
					assert.Equal(t, "2.0.0", v.MaxVer)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "bz.star")

				configContent := `
bcr = registry.http("https://bcr.bazel.build")
local = registry.file("/tmp/registry")

sync.workflow(
    name = "test",
    origin = bcr,
    destination = local,
    modules = ["rules_go"],
    ` + tt.versionCode + `
)
`
				err := os.WriteFile(configPath, []byte(configContent), 0o644)
				require.NoError(t, err)

				loader := NewLoader()
				cfg, err := loader.Load(configPath)
				require.NoError(t, err)
				require.Len(t, cfg.Workflows, 1)
				require.NotNil(t, cfg.Workflows[0].Versions)
				assert.Equal(t, tt.expectedType, cfg.Workflows[0].Versions.Type)
				tt.checkFn(t, cfg.Workflows[0].Versions)
			})
		}
	})
}

func TestLoaderLoadDefault(t *testing.T) {
	t.Parallel()
	t.Run("loads bz.star from working dir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bz.star")

		configContent := `
bcr = registry.http("https://bcr.bazel.build")
local = registry.file("/tmp/registry")

sync.workflow(
    name = "from-bz-star",
    origin = bcr,
    destination = local,
    modules = ["rules_go"],
)
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader().WithWorkingDir(tmpDir)
		cfg, err := loader.LoadDefault()
		require.NoError(t, err)
		require.Len(t, cfg.Workflows, 1)
		assert.Equal(t, "from-bz-star", cfg.Workflows[0].Name)
	})

	t.Run("loads .bz/config.star", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		bzDir := filepath.Join(tmpDir, ".bz")
		err := os.MkdirAll(bzDir, 0o755)
		require.NoError(t, err)

		configPath := filepath.Join(bzDir, "config.star")
		configContent := `
bcr = registry.http("https://bcr.bazel.build")
local = registry.file("/tmp/registry")

sync.workflow(
    name = "from-bz-config",
    origin = bcr,
    destination = local,
    modules = ["rules_go"],
)
`
		err = os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader().WithWorkingDir(tmpDir)
		cfg, err := loader.LoadDefault()
		require.NoError(t, err)
		require.Len(t, cfg.Workflows, 1)
		assert.Equal(t, "from-bz-config", cfg.Workflows[0].Name)
	})

	t.Run("bz.star takes priority over .bz/config.star", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		// Create bz.star
		bzStarPath := filepath.Join(tmpDir, "bz.star")
		bzStarContent := `
bcr = registry.http("https://bcr.bazel.build")
local = registry.file("/tmp/registry")

sync.workflow(
    name = "priority-test",
    origin = bcr,
    destination = local,
    modules = ["rules_go"],
)
`
		err := os.WriteFile(bzStarPath, []byte(bzStarContent), 0o644)
		require.NoError(t, err)

		// Create .bz/config.star
		bzDir := filepath.Join(tmpDir, ".bz")
		err = os.MkdirAll(bzDir, 0o755)
		require.NoError(t, err)

		configPath := filepath.Join(bzDir, "config.star")
		configContent := `
bcr = registry.http("https://bcr.bazel.build")
local = registry.file("/tmp/registry")

sync.workflow(
    name = "should-not-load",
    origin = bcr,
    destination = local,
    modules = ["rules_python"],
)
`
		err = os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader().WithWorkingDir(tmpDir)
		cfg, err := loader.LoadDefault()
		require.NoError(t, err)
		require.Len(t, cfg.Workflows, 1)
		assert.Equal(t, "priority-test", cfg.Workflows[0].Name)
	})

	t.Run("no config found", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		loader := NewLoader().WithWorkingDir(tmpDir)
		_, err := loader.LoadDefault()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no config file found")
	})
}

func TestConvertFunctions(t *testing.T) {
	t.Parallel()
	t.Run("RegistryFromValue nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, RegistryFromValue(nil))
	})

	t.Run("AuthFromValue nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, AuthFromValue(nil))
	})

	t.Run("WorkflowFromValue nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, WorkflowFromValue(nil))
	})

	t.Run("VersionSelectorFromValue nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, VersionSelectorFromValue(nil))
	})

	t.Run("TransformFromValue nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, TransformFromValue(nil))
	})

	t.Run("DefaultsFromValue nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, DefaultsFromValue(nil))
	})

	t.Run("AuthFromValue basic", func(t *testing.T) {
		t.Parallel()
		av := &modules.AuthValue{
			Kind:     "basic",
			Username: "user",
			Password: "pass",
		}
		auth := AuthFromValue(av)
		assert.Equal(t, "basic", auth.Type)
		assert.Equal(t, "user", auth.Username)
		assert.Equal(t, "pass", auth.Password)
	})

	t.Run("AuthFromValue bearer_token", func(t *testing.T) {
		t.Parallel()
		av := &modules.AuthValue{
			Kind:   "bearer_token",
			EnvVar: "TOKEN",
		}
		auth := AuthFromValue(av)
		assert.Equal(t, "bearer_token", auth.Type)
		assert.Equal(t, "TOKEN", auth.EnvVar)
	})

	t.Run("AuthFromValue header", func(t *testing.T) {
		t.Parallel()
		av := &modules.AuthValue{
			Kind:        "header",
			HeaderName:  "X-Custom",
			HeaderValue: "value",
		}
		auth := AuthFromValue(av)
		assert.Equal(t, "header", auth.Type)
		assert.Equal(t, "X-Custom", auth.HeaderName)
		assert.Equal(t, "value", auth.HeaderValue)
	})
}

func TestLoaderWithWorkingDir(t *testing.T) {
	t.Parallel()
	t.Run("sets working directory", func(t *testing.T) {
		t.Parallel()
		loader := NewLoader().WithWorkingDir("/custom/path")
		assert.Equal(t, "/custom/path", loader.workingDir)
	})

	t.Run("resolves relative paths from working dir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bz.star")

		configContent := `
bcr = registry.http("https://bcr.bazel.build")
local = registry.file("/tmp/registry")

sync.workflow(
    name = "test",
    origin = bcr,
    destination = local,
    modules = ["rules_go"],
)
`
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		require.NoError(t, err)

		loader := NewLoader().WithWorkingDir(tmpDir)
		cfg, err := loader.Load("bz.star")
		require.NoError(t, err)
		require.Len(t, cfg.Workflows, 1)
	})
}
