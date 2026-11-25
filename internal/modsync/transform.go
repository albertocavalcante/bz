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

	fields, err := archiveURLFields(content)
	if err != nil {
		return nil, fmt.Errorf("parse source.json: %w", err)
	}

	var output []byte
	var copied int64
	replace := func(end int64, raw json.RawMessage, allowEmpty bool) error {
		var url *string
		if err := json.Unmarshal(raw, &url); err != nil {
			return err
		}
		if url == nil || (!allowEmpty && *url == "") {
			return nil
		}
		updated := re.ReplaceAllString(*url, replacement)
		if updated == *url {
			return nil
		}
		encoded, err := json.Marshal(updated)
		if err != nil {
			return err
		}
		// Copy intervening bytes verbatim, including any ordered patch
		// entries and unknown fields.
		output = append(output, content[copied:end-int64(len(raw))]...)
		output = append(output, encoded...)
		copied = end
		return nil
	}
	for _, field := range fields {
		raw := json.RawMessage(content[field.start:field.end])
		switch field.name {
		case "url", "archive":
			err = replace(field.end, raw, false)
		case "mirror_urls", "urls":
			err = rewriteURLArray(raw, field.start, func(end int64, item json.RawMessage) error {
				return replace(end, item, field.name == "urls")
			})
		}
		if err != nil {
			return nil, fmt.Errorf("parse source.json field %q: %w", field.name, err)
		}
	}
	if copied == 0 {
		return content, nil
	}
	return append(output, content[copied:]...), nil
}

type sourceURLField struct {
	name       string
	start, end int64
}

// archiveURLFields records URL value spans without interpreting them. The
// descriptor type can appear after these fields and make them opaque.
func archiveURLFields(content []byte) ([]sourceURLField, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	open, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if open != json.Delim('{') {
		return nil, fmt.Errorf("expected an object")
	}

	const archiveSourceType = "archive"
	kind := archiveSourceType
	var fields []sourceURLField
	var raw json.RawMessage
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(&raw); err != nil {
			return nil, fmt.Errorf("field %q: %w", key, err)
		}
		name := key.(string)
		if name == "type" {
			kind = ""
			if err := json.Unmarshal(raw, &kind); err != nil {
				return nil, fmt.Errorf("field %q: %w", key, err)
			}
			continue
		}

		// Keep the historical case-insensitive match for the existing URL
		// fields. The new mirror_urls field uses the registry's exact key.
		switch normalized := strings.ToLower(name); {
		case name == "mirror_urls", normalized == "url", normalized == "archive", normalized == "urls":
			end := decoder.InputOffset()
			fields = append(fields, sourceURLField{name: normalized, start: end - int64(len(raw)), end: end})
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unexpected data after object")
	}
	if kind != archiveSourceType {
		return nil, nil
	}
	return fields, nil
}

func rewriteURLArray(raw json.RawMessage, start int64, replace func(int64, json.RawMessage) error) error {
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
		if err := replace(start+decoder.InputOffset(), item); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}
