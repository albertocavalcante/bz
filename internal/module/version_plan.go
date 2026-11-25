package module

import (
	"bytes"
	"errors"
	"fmt"
)

// VersionEdit requests a replacement for every version literal of a module
// declared in one workspace-relative file.
type VersionEdit struct {
	Rel     string
	Name    string
	Version string
}

// VersionPlan contains the file replacements and the indices of requests
// that changed bytes or were skipped because a version literal was absent.
// Each list follows first-seen file order, then request order within each file.
// Duplicate requests for the same module and file are represented by the first.
type VersionPlan struct {
	Files       []FileEdit
	Applied     []int
	Versionless []int
}

// PlanVersionEdits computes version replacements from captured file bytes.
// It leaves snapshot unchanged. If any request fails, it returns an empty plan.
// A module with a versionless declaration in its file is skipped as a whole.
func PlanVersionEdits(snapshot map[string][]byte, requests []VersionEdit) (VersionPlan, error) {
	var files []string
	byFile := make(map[string][]int)
	for i, request := range requests {
		if len(byFile[request.Rel]) == 0 {
			files = append(files, request.Rel)
		}
		byFile[request.Rel] = append(byFile[request.Rel], i)
	}

	var plan VersionPlan
	for _, file := range files {
		original, ok := snapshot[file]
		if !ok {
			return VersionPlan{}, fmt.Errorf("missing captured content for %s", file)
		}

		content := original
		seen := make(map[string]bool)
		edited := false
		for _, i := range byFile[file] {
			request := requests[i]
			if seen[request.Name] {
				continue
			}
			seen[request.Name] = true

			next, changed, err := SetDepVersion(content, request.Name, request.Version)
			if errors.Is(err, ErrNoVersionArgument) {
				plan.Versionless = append(plan.Versionless, i)
				continue
			}
			if err != nil {
				return VersionPlan{}, fmt.Errorf("failed to update %s: %w", request.Name, err)
			}
			if !changed {
				return VersionPlan{}, fmt.Errorf("failed to update %s: version %s would not change MODULE.bazel text", request.Name, request.Version)
			}

			content = next
			edited = true
			plan.Applied = append(plan.Applied, i)
		}
		if edited {
			plan.Files = append(plan.Files, FileEdit{Rel: file, Original: bytes.Clone(original), Updated: content})
		}
	}
	return plan, nil
}
