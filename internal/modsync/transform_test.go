package modsync

import (
	"encoding/json"
	"testing"

	"github.com/albertocavalcante/bz/internal/config"
)

func TestRewriteSourceURLs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		content     string
		pattern     string
		replacement string
		wantURL     string
		wantErr     bool
	}{
		{
			name: "rewrite github to mirror",
			content: `{
				"url": "https://github.com/bazelbuild/rules_go/archive/v0.50.0.tar.gz",
				"integrity": "sha256-abc123"
			}`,
			pattern:     "https://github.com",
			replacement: "https://mirror.example.com",
			wantURL:     "https://mirror.example.com/bazelbuild/rules_go/archive/v0.50.0.tar.gz",
			wantErr:     false,
		},
		{
			name: "rewrite with regex pattern",
			content: `{
				"url": "https://github.com/bazelbuild/rules_go/archive/v0.50.0.tar.gz"
			}`,
			pattern:     "github\\.com/([^/]+)/([^/]+)",
			replacement: "mirror.example.com/$1/$2",
			wantURL:     "https://mirror.example.com/bazelbuild/rules_go/archive/v0.50.0.tar.gz",
			wantErr:     false,
		},
		{
			name: "empty pattern does nothing",
			content: `{
				"url": "https://github.com/example/test.tar.gz"
			}`,
			pattern:     "",
			replacement: "replaced",
			wantURL:     "https://github.com/example/test.tar.gz",
			wantErr:     false,
		},
		{
			name: "invalid regex pattern",
			content: `{
				"url": "https://example.com"
			}`,
			pattern:     "[invalid",
			replacement: "replaced",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := rewriteSourceURLs([]byte(tt.content), tt.pattern, tt.replacement)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Parse result to check URL
			var src sourceJSON
			if err := json.Unmarshal(result, &src); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}

			if src.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", src.URL, tt.wantURL)
			}
		})
	}
}

func TestRewriteSourceURLs_URLsArray(t *testing.T) {
	t.Parallel()
	content := `{
		"urls": [
			"https://github.com/bazelbuild/rules_go/archive/v0.50.0.tar.gz",
			"https://github.com/backup/rules_go/archive/v0.50.0.tar.gz"
		],
		"integrity": "sha256-abc123"
	}`

	result, err := rewriteSourceURLs([]byte(content), "github.com", "mirror.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var src sourceJSON
	if err := json.Unmarshal(result, &src); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if len(src.URLs) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(src.URLs))
	}

	for i, url := range src.URLs {
		if url == "" {
			t.Errorf("URLs[%d] is empty", i)
		}
		if url != "https://mirror.example.com/bazelbuild/rules_go/archive/v0.50.0.tar.gz" &&
			url != "https://mirror.example.com/backup/rules_go/archive/v0.50.0.tar.gz" {
			t.Errorf("URLs[%d] = %q, not rewritten correctly", i, url)
		}
	}
}

func TestApplyTransforms(t *testing.T) {
	t.Parallel()
	content := `{
		"url": "https://github.com/example/test.tar.gz",
		"integrity": "sha256-abc123"
	}`

	transforms := []*config.Transform{
		{
			Type:        "rewrite_urls",
			Pattern:     "github.com",
			Replacement: "mirror.example.com",
		},
	}

	result, err := applyTransforms(transforms, []byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var src sourceJSON
	if err := json.Unmarshal(result, &src); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if src.URL != "https://mirror.example.com/example/test.tar.gz" {
		t.Errorf("URL = %q, want rewritten URL", src.URL)
	}
}

func TestApplyTransforms_SkipYanked(t *testing.T) {
	t.Parallel(
	// skip_yanked transform should not modify the source.json
	)

	content := `{"url": "https://example.com/test.tar.gz"}`

	transforms := []*config.Transform{
		{Type: "skip_yanked"},
	}

	result, err := applyTransforms(transforms, []byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Content should be unchanged (just re-encoded)
	var original, transformed sourceJSON
	if err := json.Unmarshal([]byte(content), &original); err != nil {
		t.Fatalf("failed to parse original: %v", err)
	}
	if err := json.Unmarshal(result, &transformed); err != nil {
		t.Fatalf("failed to parse transformed: %v", err)
	}

	if original.URL != transformed.URL {
		t.Errorf("URL changed unexpectedly: %q -> %q", original.URL, transformed.URL)
	}
}

func TestApplyTransforms_MultipleTransforms(t *testing.T) {
	t.Parallel()
	content := `{
		"url": "https://github.com/example/test.tar.gz",
		"integrity": "sha256-abc"
	}`

	transforms := []*config.Transform{
		{
			Type:        "rewrite_urls",
			Pattern:     "github.com",
			Replacement: "gitlab.com",
		},
		{
			Type:        "rewrite_urls",
			Pattern:     "gitlab.com",
			Replacement: "mirror.example.com",
		},
	}

	result, err := applyTransforms(transforms, []byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var src sourceJSON
	if err := json.Unmarshal(result, &src); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Both transforms should be applied in order
	if src.URL != "https://mirror.example.com/example/test.tar.gz" {
		t.Errorf("URL = %q, want URL with both transforms applied", src.URL)
	}
}
