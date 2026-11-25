package modsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"

	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/publisher"
	"github.com/albertocavalcante/bz/internal/registry"
)

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
		moduleData, moduleErr := dest.Read(path.Join(module, version, registry.ModuleBazelFile))
		sourceData, sourceErr := dest.Read(path.Join(module, version, registry.SourceFile))
		if moduleErr != nil && !errors.Is(moduleErr, fs.ErrNotExist) {
			return moduleErr
		}
		if sourceErr != nil && !errors.Is(sourceErr, fs.ErrNotExist) {
			return sourceErr
		}
		if moduleErr == nil && len(moduleData) > 0 && sourceErr == nil && json.Valid(sourceData) {
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
	yanked := maps.Clone(m.value.YankedVersions)
	if yanked == nil {
		yanked = make(map[string]string)
	}
	for _, version := range available {
		if reason, ok := origin.YankedVersions[version]; ok {
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
