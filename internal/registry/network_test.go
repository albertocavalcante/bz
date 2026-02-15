package registry

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/albertocavalcante/bz/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *NetworkError
		contains string
	}{
		{
			name: "basic error message",
			err: &NetworkError{
				Operation: "fetch metadata",
				URL:       "https://bcr.bazel.build",
				Err:       errors.New("connection refused"),
			},
			contains: "fetch metadata",
		},
		{
			name: "error with URL",
			err: &NetworkError{
				Operation: "get module",
				URL:       "https://bcr.bazel.build",
				Err:       errors.New("timeout"),
			},
			contains: "bcr.bazel.build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			assert.Contains(t, msg, tt.contains)
		})
	}
}

func TestNetworkError_UserFriendlyError(t *testing.T) {
	err := &NetworkError{
		Operation: "fetch metadata",
		URL:       "https://bcr.bazel.build",
		Err:       errors.New("connection refused"),
		Suggestions: []string{
			"Check your network connection",
			"Try using --prefer-offline flag",
		},
	}

	msg := err.UserFriendlyError()

	// Should contain the error icon
	assert.Contains(t, msg, "Cannot reach registry")
	// Should contain suggestions
	assert.Contains(t, msg, "Check your network connection")
	assert.Contains(t, msg, "--prefer-offline")
}

func TestNetworkError_Unwrap(t *testing.T) {
	underlying := errors.New("connection refused")
	err := &NetworkError{
		Operation: "fetch",
		URL:       "https://example.com",
		Err:       underlying,
	}

	assert.True(t, errors.Is(err, underlying))
}

// MockCacheRegistry is a mock registry that simulates a cache
type MockCacheRegistry struct {
	modules     map[string]*Metadata
	moduleBazel map[string]map[string][]byte // module -> version -> content
}

func NewMockCacheRegistry() *MockCacheRegistry {
	return &MockCacheRegistry{
		modules:     make(map[string]*Metadata),
		moduleBazel: make(map[string]map[string][]byte),
	}
}

func (m *MockCacheRegistry) GetMetadata(_ context.Context, module string) (*Metadata, error) {
	if meta, ok := m.modules[module]; ok {
		return meta, nil
	}
	return nil, ErrModuleNotFound
}

func (m *MockCacheRegistry) GetModuleBazel(_ context.Context, module, version string) ([]byte, error) {
	if versions, ok := m.moduleBazel[module]; ok {
		if content, ok := versions[version]; ok {
			return content, nil
		}
		return nil, ErrVersionNotFound
	}
	return nil, ErrModuleNotFound
}

func (m *MockCacheRegistry) ListModules(_ context.Context) ([]string, error) {
	names := make([]string, 0, len(m.modules))
	for name := range m.modules {
		names = append(names, name)
	}
	return names, nil
}

func (m *MockCacheRegistry) Type() string {
	return "cache"
}

func (m *MockCacheRegistry) String() string {
	return "cache://memory"
}

func (m *MockCacheRegistry) SetMetadata(module string, meta *Metadata) {
	m.modules[module] = meta
}

func (m *MockCacheRegistry) SetModuleBazel(module, version string, content []byte) {
	if m.moduleBazel[module] == nil {
		m.moduleBazel[module] = make(map[string][]byte)
	}
	m.moduleBazel[module][version] = content
}

// FailingRegistry always returns network errors
type FailingRegistry struct {
	err error
}

func NewFailingRegistry(err error) *FailingRegistry {
	return &FailingRegistry{err: err}
}

func (f *FailingRegistry) GetMetadata(_ context.Context, _ string) (*Metadata, error) {
	return nil, f.err
}

func (f *FailingRegistry) GetModuleBazel(_ context.Context, _, _ string) ([]byte, error) {
	return nil, f.err
}

func (f *FailingRegistry) ListModules(_ context.Context) ([]string, error) {
	return nil, f.err
}

func (f *FailingRegistry) Type() string {
	return "http"
}

func (f *FailingRegistry) String() string {
	return "https://bcr.bazel.build"
}

func TestNetworkAwareRegistry_OfflineMode_CacheHit(t *testing.T) {
	// Setup: cache has the module
	cache := NewMockCacheRegistry()
	cache.SetMetadata("rules_go", &Metadata{
		Versions: []string{"0.49.0", "0.50.0"},
	})

	inner := NewFailingRegistry(errors.New("network error"))
	opts := &cli.Options{
		Offline: true,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	// Should succeed from cache even though network would fail
	meta, err := reg.GetMetadata(ctx, "rules_go")
	require.NoError(t, err)
	assert.Equal(t, []string{"0.49.0", "0.50.0"}, meta.Versions)
}

func TestNetworkAwareRegistry_OfflineMode_CacheMiss(t *testing.T) {
	// Setup: cache is empty
	cache := NewMockCacheRegistry()
	inner := NewFailingRegistry(errors.New("network error"))
	opts := &cli.Options{
		Offline: true,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	// Should fail with helpful error
	_, err := reg.GetMetadata(ctx, "rules_go")
	require.Error(t, err)

	// Check it's a NetworkError with suggestions
	var netErr *NetworkError
	require.True(t, errors.As(err, &netErr))
	assert.Contains(t, netErr.Operation, "rules_go")
	assert.NotEmpty(t, netErr.Suggestions)

	// Check user-friendly message contains helpful info
	msg := netErr.UserFriendlyError()
	assert.Contains(t, msg, "offline mode")
	assert.Contains(t, msg, "rules_go")
}

func TestNetworkAwareRegistry_PreferOffline_CacheHit(t *testing.T) {
	// Setup: cache has the module
	cache := NewMockCacheRegistry()
	cache.SetMetadata("rules_go", &Metadata{
		Versions: []string{"0.49.0", "0.50.0"},
	})

	// Inner registry has newer version but should not be called
	callCount := 0
	inner := &MockRegistry{
		Modules: map[string]*Metadata{
			"rules_go": {Versions: []string{"0.49.0", "0.50.0", "0.51.0"}},
		},
		TypeName: "http",
	}
	// Wrap to track calls
	trackingInner := &trackingRegistry{
		inner:     inner,
		callCount: &callCount,
	}

	opts := &cli.Options{
		PreferOffline: true,
	}

	reg := NewNetworkAware(trackingInner, cache, opts)
	ctx := context.Background()

	// Should return cached version
	meta, err := reg.GetMetadata(ctx, "rules_go")
	require.NoError(t, err)
	assert.Equal(t, []string{"0.49.0", "0.50.0"}, meta.Versions)
	assert.Equal(t, 0, callCount, "inner registry should not be called")
}

func TestNetworkAwareRegistry_PreferOffline_CacheMiss_FallsBackToNetwork(t *testing.T) {
	// Setup: cache is empty
	cache := NewMockCacheRegistry()

	// Inner registry has the module
	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		PreferOffline: true,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	// Should fall back to network and succeed
	meta, err := reg.GetMetadata(ctx, "rules_go")
	require.NoError(t, err)
	assert.Equal(t, []string{"0.50.0"}, meta.Versions)
}

func TestNetworkAwareRegistry_OnlineMode_UsesNetwork(t *testing.T) {
	// Setup: cache has old version
	cache := NewMockCacheRegistry()
	cache.SetMetadata("rules_go", &Metadata{
		Versions: []string{"0.49.0"},
	})

	// Inner registry has newer version
	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.49.0", "0.50.0"}},
	})

	opts := &cli.Options{
		Offline:       false,
		PreferOffline: false,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	// Should use network (default behavior)
	meta, err := reg.GetMetadata(ctx, "rules_go")
	require.NoError(t, err)
	assert.Equal(t, []string{"0.49.0", "0.50.0"}, meta.Versions)
}

func TestNetworkAwareRegistry_GetModuleBazel_OfflineMode_CacheHit(t *testing.T) {
	cache := NewMockCacheRegistry()
	cache.SetMetadata("rules_go", &Metadata{Versions: []string{"0.50.0"}})
	cache.SetModuleBazel("rules_go", "0.50.0", []byte(`module(name = "rules_go", version = "0.50.0")`))

	inner := NewFailingRegistry(errors.New("network error"))
	opts := &cli.Options{
		Offline: true,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	content, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	require.NoError(t, err)
	assert.Contains(t, string(content), "rules_go")
}

func TestNetworkAwareRegistry_GetModuleBazel_OfflineMode_CacheMiss(t *testing.T) {
	cache := NewMockCacheRegistry()
	inner := NewFailingRegistry(errors.New("network error"))
	opts := &cli.Options{
		Offline: true,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	_, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	require.Error(t, err)

	var netErr *NetworkError
	require.True(t, errors.As(err, &netErr))
	assert.Contains(t, netErr.UserFriendlyError(), "offline mode")
}

func TestNetworkAwareRegistry_ListModules_OfflineMode(t *testing.T) {
	cache := NewMockCacheRegistry()
	cache.SetMetadata("rules_go", &Metadata{Versions: []string{"0.50.0"}})
	cache.SetMetadata("rules_python", &Metadata{Versions: []string{"0.31.0"}})

	inner := NewFailingRegistry(errors.New("network error"))
	opts := &cli.Options{
		Offline: true,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	modules, err := reg.ListModules(ctx)
	require.NoError(t, err)
	assert.Contains(t, modules, "rules_go")
	assert.Contains(t, modules, "rules_python")
}

func TestNetworkAwareRegistry_Type(t *testing.T) {
	cache := NewMockCacheRegistry()
	inner := NewMockRegistry(nil)
	inner.TypeName = "https"
	opts := &cli.Options{}

	reg := NewNetworkAware(inner, cache, opts)
	assert.Equal(t, "https", reg.Type())
}

func TestNetworkAwareRegistry_String(t *testing.T) {
	cache := NewMockCacheRegistry()
	inner := NewMockRegistry(nil)
	opts := &cli.Options{}

	reg := NewNetworkAware(inner, cache, opts)
	assert.Contains(t, reg.String(), "mock://")
}

func TestNetworkError_SuggestionsForOfflineMode(t *testing.T) {
	err := NewOfflineCacheMissError("rules_go", "https://bcr.bazel.build")

	msg := err.UserFriendlyError()

	// Should suggest helpful options
	assert.True(t, strings.Contains(msg, "cache") || strings.Contains(msg, "Cache"),
		"should mention caching")
	assert.True(t, strings.Contains(msg, "--prefer-offline") || strings.Contains(msg, "prefer-offline"),
		"should suggest prefer-offline mode")
}

func TestNetworkAwareRegistry_NilCache(t *testing.T) {
	// When cache is nil, should fallthrough to inner registry
	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		PreferOffline: true, // Even with prefer-offline, should work
	}

	reg := NewNetworkAware(inner, nil, opts)
	ctx := context.Background()

	meta, err := reg.GetMetadata(ctx, "rules_go")
	require.NoError(t, err)
	assert.Equal(t, []string{"0.50.0"}, meta.Versions)
}

func TestNetworkAwareRegistry_NilOptions(t *testing.T) {
	cache := NewMockCacheRegistry()
	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	// Nil options should default to online mode
	reg := NewNetworkAware(inner, cache, nil)
	ctx := context.Background()

	meta, err := reg.GetMetadata(ctx, "rules_go")
	require.NoError(t, err)
	assert.Equal(t, []string{"0.50.0"}, meta.Versions)
}

// trackingRegistry wraps a registry to track call counts
type trackingRegistry struct {
	inner     Registry
	callCount *int
}

func (t *trackingRegistry) GetMetadata(ctx context.Context, module string) (*Metadata, error) {
	*t.callCount++
	return t.inner.GetMetadata(ctx, module)
}

func (t *trackingRegistry) GetModuleBazel(ctx context.Context, module, version string) ([]byte, error) {
	*t.callCount++
	return t.inner.GetModuleBazel(ctx, module, version)
}

func (t *trackingRegistry) ListModules(ctx context.Context) ([]string, error) {
	*t.callCount++
	return t.inner.ListModules(ctx)
}

func (t *trackingRegistry) Type() string {
	return t.inner.Type()
}

func (t *trackingRegistry) String() string {
	return t.inner.String()
}

var _ Registry = (*trackingRegistry)(nil)

func TestNetworkError_ErrorWithoutErr(t *testing.T) {
	err := &NetworkError{
		Operation: "fetch metadata",
		URL:       "https://bcr.bazel.build",
		Err:       nil,
	}

	msg := err.Error()
	assert.Contains(t, msg, "fetch metadata")
	assert.Contains(t, msg, "bcr.bazel.build")
	// Should not contain "nil" or panic
	assert.NotContains(t, msg, "nil")
}

func TestNetworkError_UserFriendlyError_NoSuggestions(t *testing.T) {
	err := &NetworkError{
		Operation: "fetch metadata",
		URL:       "https://bcr.bazel.build",
		Err:       errors.New("timeout"),
	}

	msg := err.UserFriendlyError()
	assert.Contains(t, msg, "Cannot reach registry")
	// Should not have "Options:" section when no suggestions
	assert.NotContains(t, msg, "Options:")
}

func TestNetworkError_UserFriendlyError_NoURL(t *testing.T) {
	err := &NetworkError{
		Operation: "fetch metadata",
		Err:       errors.New("timeout"),
	}

	msg := err.UserFriendlyError()
	assert.Contains(t, msg, "Cannot reach registry")
	// Should not have empty parentheses
	assert.NotContains(t, msg, "()")
}

func TestNetworkError_UserFriendlyError_NoOperation(t *testing.T) {
	err := &NetworkError{
		URL: "https://bcr.bazel.build",
		Err: errors.New("timeout"),
		Suggestions: []string{
			"Try again later",
		},
	}

	msg := err.UserFriendlyError()
	assert.Contains(t, msg, "Cannot reach registry")
	assert.Contains(t, msg, "Try again later")
}

func TestNetworkAwareRegistry_GetModuleBazel_PreferOffline_CacheHit(t *testing.T) {
	cache := NewMockCacheRegistry()
	cache.SetModuleBazel("rules_go", "0.50.0", []byte(`module(name = "rules_go")`))

	callCount := 0
	inner := &MockRegistry{
		Modules: map[string]*Metadata{
			"rules_go": {Versions: []string{"0.50.0"}},
		},
		TypeName: "http",
	}
	trackingInner := &trackingRegistry{
		inner:     inner,
		callCount: &callCount,
	}

	opts := &cli.Options{
		PreferOffline: true,
	}

	reg := NewNetworkAware(trackingInner, cache, opts)
	ctx := context.Background()

	content, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	require.NoError(t, err)
	assert.Contains(t, string(content), "rules_go")
	assert.Equal(t, 0, callCount, "inner registry should not be called")
}

func TestNetworkAwareRegistry_GetModuleBazel_PreferOffline_CacheMiss(t *testing.T) {
	cache := NewMockCacheRegistry()
	// Cache is empty

	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		PreferOffline: true,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	// Should fall back to network
	content, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	require.NoError(t, err)
	assert.Contains(t, string(content), "rules_go")
}

func TestNetworkAwareRegistry_GetModuleBazel_OnlineMode(t *testing.T) {
	cache := NewMockCacheRegistry()
	cache.SetModuleBazel("rules_go", "0.50.0", []byte(`cached content`))

	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		Offline:       false,
		PreferOffline: false,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	// Should use network, not cache
	content, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	require.NoError(t, err)
	// MockRegistry returns its own format, not "cached content"
	assert.NotEqual(t, "cached content", string(content))
}

func TestNetworkAwareRegistry_GetModuleBazel_OfflineMode_NilCache(t *testing.T) {
	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		Offline: true,
	}

	reg := NewNetworkAware(inner, nil, opts)
	ctx := context.Background()

	_, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	require.Error(t, err)

	var netErr *NetworkError
	require.True(t, errors.As(err, &netErr))
	assert.Contains(t, netErr.UserFriendlyError(), "offline mode")
}

func TestNetworkAwareRegistry_ListModules_PreferOffline_CacheHit(t *testing.T) {
	cache := NewMockCacheRegistry()
	cache.SetMetadata("rules_go", &Metadata{Versions: []string{"0.50.0"}})

	callCount := 0
	inner := &MockRegistry{
		Modules: map[string]*Metadata{
			"rules_go": {Versions: []string{"0.50.0"}},
			"gazelle":  {Versions: []string{"0.35.0"}},
		},
		TypeName: "http",
	}
	trackingInner := &trackingRegistry{
		inner:     inner,
		callCount: &callCount,
	}

	opts := &cli.Options{
		PreferOffline: true,
	}

	reg := NewNetworkAware(trackingInner, cache, opts)
	ctx := context.Background()

	modules, err := reg.ListModules(ctx)
	require.NoError(t, err)
	assert.Contains(t, modules, "rules_go")
	assert.Equal(t, 0, callCount, "inner registry should not be called")
}

func TestNetworkAwareRegistry_ListModules_PreferOffline_CacheMiss(t *testing.T) {
	cache := NewMockCacheRegistry()
	// Cache is empty

	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		PreferOffline: true,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	// Should fall back to network
	modules, err := reg.ListModules(ctx)
	require.NoError(t, err)
	assert.Contains(t, modules, "rules_go")
}

func TestNetworkAwareRegistry_ListModules_OnlineMode(t *testing.T) {
	cache := NewMockCacheRegistry()
	cache.SetMetadata("cached_module", &Metadata{Versions: []string{"1.0.0"}})

	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		Offline:       false,
		PreferOffline: false,
	}

	reg := NewNetworkAware(inner, cache, opts)
	ctx := context.Background()

	// Should use network, not cache
	modules, err := reg.ListModules(ctx)
	require.NoError(t, err)
	assert.Contains(t, modules, "rules_go")
	assert.NotContains(t, modules, "cached_module")
}

func TestNetworkAwareRegistry_ListModules_OfflineMode_NilCache(t *testing.T) {
	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		Offline: true,
	}

	reg := NewNetworkAware(inner, nil, opts)
	ctx := context.Background()

	_, err := reg.ListModules(ctx)
	require.Error(t, err)

	var netErr *NetworkError
	require.True(t, errors.As(err, &netErr))
	assert.Contains(t, netErr.UserFriendlyError(), "offline mode")
}

func TestNewOfflineVersionMissError(t *testing.T) {
	err := NewOfflineVersionMissError("rules_go", "0.50.0", "https://bcr.bazel.build")

	assert.Contains(t, err.Operation, "rules_go@0.50.0")
	assert.Contains(t, err.Operation, "offline mode")
	assert.Equal(t, "https://bcr.bazel.build", err.URL)
	assert.True(t, errors.Is(err, ErrVersionNotFound))
	assert.NotEmpty(t, err.Suggestions)
}

func TestNetworkAwareRegistry_GetModuleBazel_PreferOffline_NilCache(t *testing.T) {
	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		PreferOffline: true,
	}

	// With nil cache, should fall through to network
	reg := NewNetworkAware(inner, nil, opts)
	ctx := context.Background()

	content, err := reg.GetModuleBazel(ctx, "rules_go", "0.50.0")
	require.NoError(t, err)
	assert.Contains(t, string(content), "rules_go")
}

func TestNetworkAwareRegistry_ListModules_PreferOffline_NilCache(t *testing.T) {
	inner := NewMockRegistry(map[string]*Metadata{
		"rules_go": {Versions: []string{"0.50.0"}},
	})

	opts := &cli.Options{
		PreferOffline: true,
	}

	// With nil cache, should fall through to network
	reg := NewNetworkAware(inner, nil, opts)
	ctx := context.Background()

	modules, err := reg.ListModules(ctx)
	require.NoError(t, err)
	assert.Contains(t, modules, "rules_go")
}
