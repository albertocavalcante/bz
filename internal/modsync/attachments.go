package modsync

import (
	"bytes"
	"context"
	"crypto"
	_ "crypto/sha1"   //nolint:gosec // Accept legacy SHA-1 integrity references supported by Bazel.
	_ "crypto/sha256" // Register the SHA-256 constructor for crypto.Hash.
	_ "crypto/sha512" // Register SHA-384 and SHA-512 for crypto.Hash.
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/publisher"
	"github.com/albertocavalcante/bz/internal/registry"
)

type versionAttachment struct {
	path     string
	hash     crypto.Hash
	checksum []byte
}

// inspectMirrorSource validates the descriptor and reads attachment references
// without rewriting it. Bazel applies patches in the original JSON order.
func inspectMirrorSource(module, version string, source []byte) ([]versionAttachment, error) {
	var descriptor map[string]json.RawMessage
	if err := json.Unmarshal(source, &descriptor); err != nil {
		return nil, fmt.Errorf("parse source.json: %w", err)
	}
	if descriptor == nil {
		return nil, fmt.Errorf("source.json is missing or not a JSON object")
	}
	var sourceType string
	if raw, ok := descriptor["type"]; ok {
		if err := json.Unmarshal(raw, &sourceType); err != nil {
			return nil, fmt.Errorf("parse source.json type: %w", err)
		}
	}
	type attachmentGroup struct {
		dir    string
		fields json.RawMessage
	}
	var groups []attachmentGroup
	switch sourceType {
	case "", "archive":
		groups = append(groups, attachmentGroup{"overlay", descriptor["overlay"]})
	case "git_repository":
		// Git-backed versions can also have patches.
	default:
		return nil, nil
	}
	groups = append(groups, attachmentGroup{"patches", descriptor["patches"]})

	var attachments []versionAttachment
	for _, group := range groups {
		if len(group.fields) == 0 {
			continue
		}
		var files map[string]string
		if err := json.Unmarshal(group.fields, &files); err != nil {
			return nil, fmt.Errorf("parse source.json %s: %w", group.dir, err)
		}
		for _, name := range slices.Sorted(maps.Keys(files)) {
			relative := group.dir + "/" + name
			if err := coord.ValidateRegistryPath(module + "/" + version + "/" + relative); err != nil {
				return nil, err
			}
			hash, checksum, err := parseIntegrity(files[name])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", relative, err)
			}
			attachments = append(attachments, versionAttachment{path: relative, hash: hash, checksum: checksum})
		}
	}
	return attachments, nil
}

func parseIntegrity(integrity string) (crypto.Hash, []byte, error) {
	algorithm, encoded, _ := strings.Cut(integrity, "-")
	var hash crypto.Hash
	switch algorithm {
	case "sha1":
		hash = crypto.SHA1
	case "sha256":
		hash = crypto.SHA256
	case "sha384":
		hash = crypto.SHA384
	case "sha512":
		hash = crypto.SHA512
	default:
		return 0, nil, fmt.Errorf("unsupported attachment integrity algorithm")
	}
	if strings.ContainsAny(encoded, "\r\n") {
		return 0, nil, fmt.Errorf("invalid %s attachment integrity", algorithm)
	}
	checksum, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		checksum, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil || len(checksum) != hash.Size() {
		return 0, nil, fmt.Errorf("invalid %s attachment integrity", algorithm)
	}
	return hash, checksum, nil
}

func (a versionAttachment) verify(data []byte) error {
	hash := a.hash.New()
	_, _ = hash.Write(data)
	if !bytes.Equal(hash.Sum(nil), a.checksum) {
		return fmt.Errorf("%s: attachment integrity mismatch", a.path)
	}
	return nil
}

func (a versionAttachment) read(ctx context.Context, source registry.VersionFileGetter, module, version string) ([]byte, error) {
	data, err := source.GetVersionFile(ctx, module, version, a.path)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", a.path, err)
	}
	if err := a.verify(data); err != nil {
		return nil, err
	}
	return data, nil
}

func missingMirrorAttachments(dest *publisher.FilePublisher, module, version string, attachments []versionAttachment) ([]versionAttachment, error) {
	prefix := module + "/" + version + "/"
	var missing []versionAttachment
	for _, attachment := range attachments {
		data, err := dest.Read(prefix + attachment.path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read %s: %w", attachment.path, err)
		}
		if err != nil || attachment.verify(data) != nil {
			missing = append(missing, attachment)
		}
	}
	return missing, nil
}

// syncMirrorAttachments copies descriptor-referenced files before the
// version is advertised in metadata.json. Complete local files are reused.
func (r *runner) syncMirrorAttachments(ctx context.Context, module, version string, attachments []versionAttachment) (bool, error) {
	missing, err := missingMirrorAttachments(r.local, module, version, attachments)
	if err != nil || len(missing) == 0 {
		return false, err
	}
	if r.opts.DryRun {
		return true, nil
	}
	source, ok := r.source.(registry.VersionFileGetter)
	if !ok {
		return false, fmt.Errorf("origin does not support fetching registry attachments")
	}
	prefix := module + "/" + version + "/"
	for _, attachment := range missing {
		data, err := attachment.read(ctx, source, module, version)
		if err != nil {
			return false, err
		}
		if err := r.local.Put(ctx, prefix+attachment.path, data); err != nil {
			return false, fmt.Errorf("publish %s: %w", attachment.path, err)
		}
	}
	// Recheck all files after publishing. Distinct reference paths can alias
	// on filesystems with case-insensitive names.
	remaining, err := missingMirrorAttachments(r.local, module, version, attachments)
	if err != nil {
		return false, err
	}
	if len(remaining) != 0 {
		return false, fmt.Errorf("published attachment %s failed verification", remaining[0].path)
	}
	return true, nil
}
