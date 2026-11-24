package osv

import (
	"fmt"
	"strings"
)

// Ecosystems lists the names OSV accepts, copied on 2026-11-24 from the
// "Defined ecosystems" table at https://ossf.github.io/osv-schema/#affectedpackage-field.
// OSV matches ecosystem names exactly, so a misspelled one silently matches nothing.
var Ecosystems = []string{
	"AlmaLinux", "Alpaquita", "Alpine", "Android", "Azure Linux", "BellSoft Hardened Containers",
	"Bioconductor", "Bitnami", "Chainguard", "CleanStart", "ConanCenter", "CRAN", "crates.io",
	"Debian", "Docker Hardened Images", "Echo", "FreeBSD", "GHC", "GitHub Actions", "Go",
	"Hackage", "Hex", "Homebrew", "Julia", "Kubernetes", "Linux", "Mageia", "Maven", "MinimOS",
	"npm", "NuGet", "opam", "openEuler", "openSUSE", "OSS-Fuzz", "Packagist", "Photon OS", "Pub",
	"PyPI", "Red Hat", "Red Hat Lightwell", "Rocky Linux", "Root", "RubyGems", "SUSE", "SwiftURL",
	"TuxCare", "Ubuntu", "vcpkg", "VSCode", "Wolfi", "WordPress",
}

// ParseEcosystem returns the canonical spelling of an ecosystem name, matched
// case-insensitively. A ":<release>" suffix (e.g. "Debian:12") is kept as given.
func ParseEcosystem(s string) (string, error) {
	base, suffix, hasSuffix := strings.Cut(s, ":")
	for _, e := range Ecosystems {
		if strings.EqualFold(base, e) {
			if hasSuffix {
				return e + ":" + suffix, nil
			}
			return e, nil
		}
	}
	const list = "https://ossf.github.io/osv-schema/#affectedpackage-field"
	if near := nearestEcosystem(base); near != "" {
		return "", fmt.Errorf("unknown OSV ecosystem %q: did you mean %q? (valid names: %s)", s, near, list)
	}
	return "", fmt.Errorf("unknown OSV ecosystem %q (valid names: %s)", s, list)
}

// nearestEcosystem returns the closest ecosystem within two edits, or "".
func nearestEcosystem(s string) string {
	best, bestDist := "", 3
	for _, e := range Ecosystems {
		if d := editDistance(strings.ToLower(s), strings.ToLower(e)); d < bestDist {
			best, bestDist = e, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
