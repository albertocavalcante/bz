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
