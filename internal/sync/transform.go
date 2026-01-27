package sync

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/albertocavalcante/bz/internal/config"
)

// applyTransforms applies workflow transformations to source.json content.
func applyTransforms(transforms []*config.Transform, sourceJSON []byte) ([]byte, error) {
	result := sourceJSON

	for _, t := range transforms {
		var err error
		switch t.Type {
		case "rewrite_urls":
			result, err = rewriteSourceURLs(result, t.Pattern, t.Replacement)
			if err != nil {
				return nil, fmt.Errorf("rewrite_urls: %w", err)
			}

		case "skip_yanked":
			// This is handled at the version selection level, not here

		case "include_patches":
			// This is handled at the file copying level, not here
			// The transform is a flag that tells us whether to copy patches

		default:
			// Unknown transform, skip
		}
	}

	return result, nil
}

// sourceJSON represents the structure of a source.json file.
type sourceJSON struct {
	URL         string            `json:"url,omitempty"`
	Integrity   string            `json:"integrity,omitempty"`
	StripPrefix string            `json:"strip_prefix,omitempty"`
	Patches     map[string]string `json:"patches,omitempty"`
	PatchStrip  int               `json:"patch_strip,omitempty"`
	Overlay     map[string]string `json:"overlay,omitempty"`
	// Archive type fields
	Type    string   `json:"type,omitempty"`
	URLs    []string `json:"urls,omitempty"`
	Archive string   `json:"archive,omitempty"`
}

// rewriteSourceURLs rewrites URLs in source.json matching a pattern.
func rewriteSourceURLs(content []byte, pattern, replacement string) ([]byte, error) {
	if pattern == "" {
		return content, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}

	// Parse the source.json
	var src sourceJSON
	if err := json.Unmarshal(content, &src); err != nil {
		return nil, fmt.Errorf("parse source.json: %w", err)
	}

	// Rewrite single URL field
	if src.URL != "" {
		src.URL = re.ReplaceAllString(src.URL, replacement)
	}

	// Rewrite URLs array
	for i, url := range src.URLs {
		src.URLs[i] = re.ReplaceAllString(url, replacement)
	}

	// Rewrite Archive field
	if src.Archive != "" {
		src.Archive = re.ReplaceAllString(src.Archive, replacement)
	}

	// Re-encode to JSON with indentation for readability
	result, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode source.json: %w", err)
	}

	return result, nil
}
