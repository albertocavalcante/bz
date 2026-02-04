package registry

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectedType string
		wantErr      bool
	}{
		{
			name:         "https URL",
			url:          "https://bcr.bazel.build",
			expectedType: TypeHTTPS,
		},
		{
			name:         "http URL",
			url:          "http://example.com/registry",
			expectedType: TypeHTTP,
		},
		{
			name:         "file URL",
			url:          "file:///path/to/registry",
			expectedType: TypeFile,
		},
		{
			name:         "absolute path (unix)",
			url:          "/path/to/registry",
			expectedType: TypeFile,
		},
		// Windows absolute paths
		{
			name:         "windows path with backslash",
			url:          `C:\path\to\registry`,
			expectedType: TypeFile,
		},
		{
			name:         "windows path with forward slash",
			url:          "C:/path/to/registry",
			expectedType: TypeFile,
		},
		{
			name:         "windows path lowercase drive",
			url:          `d:\registry`,
			expectedType: TypeFile,
		},
		{
			name:         "windows path mixed slashes",
			url:          `E:\path/to/registry`,
			expectedType: TypeFile,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid scheme",
			url:     "ftp://example.com",
			wantErr: true,
		},
		{
			name:    "relative path",
			url:     "relative/path",
			wantErr: true,
		},
		// Windows relative paths (should NOT match)
		{
			name:    "windows relative path",
			url:     "C:relative",
			wantErr: true,
		},
		{
			name:    "just drive letter",
			url:     "C:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := New(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Error("New() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			if got := reg.Type(); got != tt.expectedType {
				t.Errorf("Type() = %q, want %q", got, tt.expectedType)
			}
		})
	}
}

func TestDefaultRegistry(t *testing.T) {
	reg := Default()
	if reg.Type() != TypeHTTPS {
		t.Errorf("Default().Type() = %q, want %q", reg.Type(), TypeHTTPS)
	}
	if reg.String() != DefaultBCR {
		t.Errorf("Default().String() = %q, want %q", reg.String(), DefaultBCR)
	}
}

func TestNewWithOptions(t *testing.T) {
	t.Run("https with cache", func(t *testing.T) {
		cacheDir := t.TempDir()
		reg, err := NewWithOptions("https://bcr.bazel.build", WithCacheDir(cacheDir))
		if err != nil {
			t.Fatalf("NewWithOptions() error = %v", err)
		}
		if got := reg.Type(); got != TypeHTTPS {
			t.Errorf("Type() = %q, want %q", got, TypeHTTPS)
		}
	})

	t.Run("file registry", func(t *testing.T) {
		reg, err := NewWithOptions("file:///path/to/registry")
		if err != nil {
			t.Fatalf("NewWithOptions() error = %v", err)
		}
		if got := reg.Type(); got != TypeFile {
			t.Errorf("Type() = %q, want %q", got, TypeFile)
		}
	})

	t.Run("windows path", func(t *testing.T) {
		reg, err := NewWithOptions(`C:\path\to\registry`)
		if err != nil {
			t.Fatalf("NewWithOptions() error = %v", err)
		}
		if got := reg.Type(); got != TypeFile {
			t.Errorf("Type() = %q, want %q", got, TypeFile)
		}
	})

	t.Run("invalid url", func(t *testing.T) {
		_, err := NewWithOptions("invalid://url")
		if err == nil {
			t.Error("expected error for invalid URL")
		}
	})
}

func TestIsWindowsAbsolutePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Valid Windows absolute paths
		{`C:\`, true},
		{`C:\path`, true},
		{`C:\path\to\registry`, true},
		{"C:/", true},
		{"C:/path", true},
		{"C:/path/to/registry", true},
		{`D:\registry`, true},
		{"d:/registry", true},
		{`Z:\some\path`, true},
		{`a:\lowercase`, true},
		{`E:\path/mixed/slashes`, true},

		// Invalid - not absolute paths
		{"", false},
		{"C", false},
		{"C:", false},
		{"C:path", false},     // Relative to current dir on C:
		{"C:path/to", false},  // Still relative
		{"/unix/path", false}, // Unix path, not Windows
		{"relative/path", false},
		{"./relative", false},
		{"../parent", false},
		{`\\server\share`, false}, // UNC path - not handled (yet)
		{"1:/invalid", false},     // Invalid drive letter
		{"@:/invalid", false},     // Invalid drive letter
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isWindowsAbsolutePath(tt.path)
			if got != tt.want {
				t.Errorf("isWindowsAbsolutePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
