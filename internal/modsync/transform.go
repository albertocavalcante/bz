package modsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

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

// rewriteSourceURLs rewrites only the URL strings in a source descriptor.
func rewriteSourceURLs(content []byte, pattern, replacement string) ([]byte, error) {
	if pattern == "" {
		return content, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	open, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("parse source.json: %w", err)
	}
	if open != json.Delim('{') {
		return nil, fmt.Errorf("parse source.json: expected an object")
	}

	var output []byte
	var copied int
	replace := func(end int, raw json.RawMessage) error {
		var url string
		if err := json.Unmarshal(raw, &url); err != nil {
			return err
		}
		if url == "" {
			return nil
		}
		updated := re.ReplaceAllString(url, replacement)
		if updated == url {
			return nil
		}
		encoded, err := json.Marshal(updated)
		if err != nil {
			return err
		}
		// InputOffset points just after the raw value. Copy intervening bytes
		// verbatim, including any ordered patch entries and unknown fields.
		output = append(output, content[copied:end-len(raw)]...)
		output = append(output, encoded...)
		copied = end
		return nil
	}

	var raw json.RawMessage
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("parse source.json: %w", err)
		}
		if err := decoder.Decode(&raw); err != nil {
			return nil, fmt.Errorf("parse source.json field %q: %w", key, err)
		}
		end := int(decoder.InputOffset())
		switch strings.ToLower(key.(string)) {
		case "url", "archive":
			err = replace(end, raw)
		case "mirror_urls", "urls":
			err = rewriteURLArray(raw, end-len(raw), replace)
		}
		if err != nil {
			return nil, fmt.Errorf("parse source.json field %q: %w", key, err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("parse source.json: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("parse source.json: %w", err)
		}
		return nil, fmt.Errorf("parse source.json: unexpected data after object")
	}
	if copied == 0 {
		return content, nil
	}
	return append(output, content[copied:]...), nil
}

func rewriteURLArray(raw json.RawMessage, start int, replace func(int, json.RawMessage) error) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	open, err := decoder.Token()
	if err != nil || open == nil {
		return err
	}
	if open != json.Delim('[') {
		return fmt.Errorf("expected an array of URLs")
	}
	var item json.RawMessage
	for decoder.More() {
		if err := decoder.Decode(&item); err != nil {
			return err
		}
		if err := replace(start+int(decoder.InputOffset()), item); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}
