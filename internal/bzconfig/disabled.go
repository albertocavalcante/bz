package bzconfig

import (
	"fmt"
	"slices"
	"strings"
)

// DisabledSet is the resolved form of [commands] disabled: command paths
// (without the root, e.g. "registry ping") mapped to the entry that
// disabled them.
type DisabledSet struct {
	paths map[string]string
}

// Match reports whether path or one of its ancestors is disabled, returning
// the configured entry responsible.
func (d DisabledSet) Match(path string) (string, bool) {
	for p := path; p != ""; {
		if entry, ok := d.paths[p]; ok {
			return entry, true
		}
		i := strings.LastIndex(p, " ")
		if i < 0 {
			break
		}
		p = p[:i]
	}
	return "", false
}

// Len returns the number of disabled command paths.
func (d DisabledSet) Len() int { return len(d.paths) }

// normalizeCommandEntry collapses whitespace and drops a leading "bz".
func normalizeCommandEntry(entry string) string {
	fields := strings.Fields(entry)
	if len(fields) > 1 && fields[0] == "bz" {
		fields = fields[1:]
	}
	return strings.Join(fields, " ")
}

// ResolveDisabled maps configured entries onto the known command paths.
//
// An entry is a full command path without the root ("registry ping",
// "cache", "mod add"); disabling a parent disables its children. A single
// word that is not a full path is accepted as an alias for the one command
// with that name ("ping" for "registry ping"). A bare name shared by several
// commands ("stats") is an error listing the candidates. Entries that match
// no command, or name a protected command, produce warnings. source names
// where the list came from, for messages.
func ResolveDisabled(entries, commands []string, source Source) (DisabledSet, []string, error) {
	set := DisabledSet{paths: map[string]string{}}
	known := make(map[string]bool, len(commands))
	for _, c := range commands {
		known[c] = true
	}

	var warnings, ambiguous []string
	for _, raw := range entries {
		entry := normalizeCommandEntry(raw)
		if entry == "" {
			continue
		}
		if IsProtectedCommand(entry) {
			warnings = append(warnings, fmt.Sprintf(
				"[commands] disabled entry %q in %s ignored: %q cannot be disabled", raw, source, entry))
			continue
		}
		if known[entry] {
			set.paths[entry] = raw
			continue
		}
		if strings.Contains(entry, " ") {
			warnings = append(warnings, fmt.Sprintf(
				"[commands] disabled entry %q in %s matches no command", raw, source))
			continue
		}
		var candidates []string
		for _, c := range commands {
			if IsProtectedCommand(c) {
				continue
			}
			if c == entry || strings.HasSuffix(c, " "+entry) {
				candidates = append(candidates, c)
			}
		}
		switch len(candidates) {
		case 0:
			warnings = append(warnings, fmt.Sprintf(
				"[commands] disabled entry %q in %s matches no command", raw, source))
		case 1:
			set.paths[candidates[0]] = raw
		default:
			slices.Sort(candidates)
			quoted := make([]string, len(candidates))
			for i, c := range candidates {
				quoted[i] = fmt.Sprintf("%q", c)
			}
			ambiguous = append(ambiguous, fmt.Sprintf("%q matches %s", raw, strings.Join(quoted, ", ")))
		}
	}
	if len(ambiguous) > 0 {
		return DisabledSet{}, warnings, fmt.Errorf(
			"ambiguous [commands] disabled entry in %s: %s; use the full command path",
			source, strings.Join(ambiguous, "; "))
	}
	return set, warnings, nil
}
