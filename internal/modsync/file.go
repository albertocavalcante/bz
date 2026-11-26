package modsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"

	bazelversion "github.com/albertocavalcante/go-bzlmod/selection/version"
	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/bz/internal/coord"
	"github.com/albertocavalcante/bz/internal/publisher"
	"github.com/albertocavalcante/bz/internal/registry"
)

// normalizedVersion follows Bazel's version equality, including validation.
func normalizedVersion(value string) (string, bool) {
	parsed, err := bazelversion.Parse(value)
	return parsed.Normalized, err == nil && !parsed.IsEmpty
}

func validModule(data []byte, name, version string) bool {
	parsed, err := build.ParseModule("MODULE.bazel", data)
	if err != nil {
		return false
	}
	expected, ok := normalizedVersion(version)
	if !ok {
		return false
	}
	var calls int
	for _, stmt := range parsed.Stmt {
		call, ok := stmt.(*build.CallExpr)
		if !ok {
			continue
		}
		id, ok := call.X.(*build.Ident)
		if !ok || id.Name != "module" {
			continue
		}
		calls++
		seenName, seenVersion := false, false
		args, expansion := expandedArgs(call.List)
		for _, arg := range args {
			assign, ok := arg.(*build.AssignExpr)
			if !ok {
				continue
			}
			key, ok := assign.LHS.(*build.Ident)
			if !ok {
				expansion = true
				continue
			}
			if key.Name != "name" && key.Name != "version" {
				continue
			}
			if key.Name == "name" {
				if seenName {
					return false
				}
				seenName = true
			} else {
				if seenVersion {
					return false
				}
				seenVersion = true
			}
			value, literal := assign.RHS.(*build.StringExpr)
			if !literal {
				continue
			} // Bazel evaluates and validates dynamic values.
			if key.Name == "name" && value.Value != name {
				return false
			}
			if key.Name == "version" {
				actual, valid := normalizedVersion(value.Value)
				if !valid || actual != expected {
					return false
				}
			}
		}
		if (!seenName || !seenVersion) && !expansion {
			return false
		}
	}
	return calls == 1
}

// expandedArgs checks literal dictionary expansions and leaves dynamic ones to Bazel.
func expandedArgs(input []build.Expr) ([]build.Expr, bool) {
	args := make([]build.Expr, 0, len(input))
	dynamic := false
	for _, arg := range input {
		unary, ok := arg.(*build.UnaryExpr)
		if !ok || unary.Op != "**" {
			args = append(args, arg)
			continue
		}
		dict, ok := unary.X.(*build.DictExpr)
		if !ok {
			dynamic = true
			continue
		}
		for _, entry := range dict.List {
			key, ok := entry.Key.(*build.StringExpr)
			if !ok {
				dynamic = true
				continue
			}
			args = append(args, &build.AssignExpr{LHS: &build.Ident{Name: key.Value}, RHS: entry.Value})
		}
	}
	return args, dynamic
}

// yankReason matches normalized Bazel versions, preferring exact keys.
func yankReason(yanks map[string]string, version string) (string, bool) {
	if reason, ok := yanks[version]; ok {
		return reason, true
	}
	normalized, valid := normalizedVersion(version)
	if !valid {
		return "", false
	}
	keys := slices.Sorted(maps.Keys(yanks))
	for _, key := range keys {
		if candidate, ok := normalizedVersion(key); ok && candidate == normalized {
			return yanks[key], true
		}
	}
	return "", false
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

func reconcileYanks(origin *registry.Metadata, prior map[string]string, versions []string) map[string]string {
	yanked := make(map[string]string)
	originVersions := make(map[string]bool, len(origin.Versions))
	for _, version := range origin.Versions {
		if normalized, ok := normalizedVersion(version); ok {
			originVersions[normalized] = true
		}
	}
	for _, version := range versions {
		normalized, valid := normalizedVersion(version)
		if valid && originVersions[normalized] {
			if reason, ok := yankReason(origin.YankedVersions, version); ok {
				yanked[version] = reason
			}
		} else if reason, ok := yankReason(prior, version); ok {
			yanked[version] = reason
		}
	}
	return yanked
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
		moduleOK, err := completeFile(dest, path.Join(module, version, registry.ModuleBazelFile), func(data []byte) bool { return validModule(data, module, version) })
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
	yanked := reconcileYanks(origin, m.value.YankedVersions, versions)
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
