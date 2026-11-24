package version

import "runtime/debug"

// Placeholder values that mean "not set by -ldflags".
const (
	// UnsetVersion is the default version string of a build without ldflags.
	UnsetVersion = "dev"
	// UnsetCommit is the default commit string of a build without ldflags.
	UnsetCommit = "none"
	// UnsetDate is the default build date of a build without ldflags.
	UnsetDate = "unknown"
)

const shortCommitLen = 7

// Info describes the running binary.
type Info struct {
	Version string
	Commit  string
	Date    string
}

// Resolve merges link-time values with the build metadata embedded by the Go
// toolchain. A link-time value (-ldflags -X) wins when it differs from its
// placeholder; otherwise the module version and vcs.* settings from bi are
// used, which is what `go install module@version` and `go build` in a VCS
// checkout provide. bi may be nil.
func Resolve(ldVersion, ldCommit, ldDate string, bi *debug.BuildInfo) Info {
	info := Info{Version: ldVersion, Commit: ldCommit, Date: ldDate}
	if bi == nil {
		return info
	}

	if isUnset(info.Version, UnsetVersion) {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			info.Version = v
		}
	}

	var revision, vcsTime string
	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if isUnset(info.Commit, UnsetCommit) && revision != "" {
		if len(revision) > shortCommitLen {
			revision = revision[:shortCommitLen]
		}
		if modified {
			revision += "-dirty"
		}
		info.Commit = revision
	}
	if isUnset(info.Date, UnsetDate) && vcsTime != "" {
		info.Date = vcsTime
	}
	return info
}

func isUnset(value, placeholder string) bool {
	return value == "" || value == placeholder
}

// Current resolves Info for the running binary from the given link-time values.
func Current(ldVersion, ldCommit, ldDate string) Info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		bi = nil
	}
	return Resolve(ldVersion, ldCommit, ldDate, bi)
}
