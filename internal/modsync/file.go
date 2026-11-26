package modsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/publisher"
	"github.com/albertocavalcante/bz/internal/registry"
)

func validModule(data []byte) bool {
	if len(bytes.TrimSpace(data)) == 0 {
		return false
	}
	_, err := build.ParseModule("MODULE.bazel", data)
	return err == nil
}

func validSource(data []byte) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal(data, &object) == nil && object != nil
}

func completeFile(dest *publisher.FilePublisher, name string, valid func([]byte) bool) (bool, error) {
	data, err := dest.Read(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return valid(data), nil
}

type localMetadata struct {
	fields map[string]json.RawMessage
	value  registry.Metadata
}

func readLocalMetadata(dest *publisher.FilePublisher, module string) (*localMetadata, error) {
	data, err := dest.Read(path.Join(module, registry.MetadataFile))
	if errors.Is(err, fs.ErrNotExist) {
		return &localMetadata{}, nil
	}
	if err != nil {
		return nil, err
	}

	metadata := &localMetadata{}
	if err := json.Unmarshal(data, &metadata.fields); err != nil {
		return nil, err
	}
	if metadata.fields == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	if err := json.Unmarshal(data, &metadata.value); err != nil {
		return nil, err
	}
	return metadata, nil
}

func (m *localMetadata) write(dest *publisher.FilePublisher, module string, origin *registry.Metadata, available []string) error {
	known := make(map[string]bool, len(available)+len(m.value.Versions))
	for _, version := range available {
		known[version] = true
	}
	for _, version := range m.value.Versions {
		if coord.ValidateVersion(version) != nil {
			continue
		}
		moduleOK, err := completeFile(dest, path.Join(module, version, registry.ModuleBazelFile), validModule)
		if err != nil {
			return err
		}
		sourceOK, err := completeFile(dest, path.Join(module, version, registry.SourceFile), validSource)
		if err != nil {
			return err
		}
		if moduleOK && sourceOK {
			known[version] = true
		}
	}
	versions := make([]string, 0, len(known))
	// Follow source order where known. Carry forward destination-only versions
	// in their prior relative order.
	for _, catalog := range [][]string{origin.Versions, m.value.Versions} {
		for _, version := range catalog {
			if known[version] {
				versions = append(versions, version)
				delete(known, version)
			}
		}
	}
	yanked := make(map[string]string)
	originVersions := make(map[string]bool, len(origin.Versions))
	for _, version := range origin.Versions {
		originVersions[version] = true
	}
	for _, version := range versions {
		if originVersions[version] {
			if reason, ok := origin.YankedVersions[version]; ok {
				yanked[version] = reason
			}
		} else if reason, ok := m.value.YankedVersions[version]; ok {
			yanked[version] = reason
		}
	}
	if slices.Equal(versions, m.value.Versions) && maps.Equal(yanked, m.value.YankedVersions) {
		return nil
	}

	if m.fields == nil {
		data, err := json.Marshal(origin)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &m.fields); err != nil {
			return err
		}
	}
	versionData, err := json.Marshal(versions)
	if err != nil {
		return err
	}
	m.fields["versions"] = versionData
	if len(yanked) == 0 {
		delete(m.fields, "yanked_versions")
	} else {
		yankedData, err := json.Marshal(yanked)
		if err != nil {
			return err
		}
		m.fields["yanked_versions"] = yankedData
	}

	data, err := json.MarshalIndent(m.fields, "", "  ")
	if err != nil {
		return err
	}
	return dest.Replace(path.Join(module, registry.MetadataFile), append(data, '\n'))
}
