package version

import (
	"runtime/debug"
	"testing"
)

func buildInfo(mainVersion string, settings ...string) *debug.BuildInfo {
	bi := &debug.BuildInfo{Main: debug.Module{Version: mainVersion}}
	for i := 0; i+1 < len(settings); i += 2 {
		bi.Settings = append(bi.Settings, debug.BuildSetting{Key: settings[i], Value: settings[i+1]})
	}
	return bi
}

func TestResolve(t *testing.T) {
	const rev = "0123456789abcdef0123456789abcdef01234567"
	tests := []struct {
		name string
		ld   [3]string
		bi   *debug.BuildInfo
		want Info
	}{
		{
			name: "ldflags win over build info",
			ld:   [3]string{"1.2.3", "abc1234", "2026-01-01T00:00:00Z"},
			bi:   buildInfo("v9.9.9", "vcs.revision", rev, "vcs.time", "2020-01-01T00:00:00Z"),
			want: Info{"1.2.3", "abc1234", "2026-01-01T00:00:00Z"},
		},
		{
			name: "nil build info keeps placeholders",
			ld:   [3]string{UnsetVersion, UnsetCommit, UnsetDate},
			bi:   nil,
			want: Info{UnsetVersion, UnsetCommit, UnsetDate},
		},
		{
			name: "go install module version",
			ld:   [3]string{UnsetVersion, UnsetCommit, UnsetDate},
			bi:   buildInfo("v0.3.1"),
			want: Info{"v0.3.1", UnsetCommit, UnsetDate},
		},
		{
			name: "devel module version is ignored",
			ld:   [3]string{UnsetVersion, UnsetCommit, UnsetDate},
			bi:   buildInfo("(devel)", "vcs.revision", rev, "vcs.time", "2026-02-03T04:05:06Z", "vcs.modified", "false"),
			want: Info{UnsetVersion, "0123456", "2026-02-03T04:05:06Z"},
		},
		{
			name: "modified tree is marked dirty",
			ld:   [3]string{UnsetVersion, UnsetCommit, UnsetDate},
			bi:   buildInfo("v0.3.2-0.20260203040506-0123456789ab+dirty", "vcs.revision", rev, "vcs.modified", "true"),
			want: Info{"v0.3.2-0.20260203040506-0123456789ab+dirty", "0123456-dirty", UnsetDate},
		},
		{
			name: "fields fall back independently",
			ld:   [3]string{"nightly.20260101.abc1234", UnsetCommit, UnsetDate},
			bi:   buildInfo("(devel)", "vcs.revision", "abc"),
			want: Info{"nightly.20260101.abc1234", "abc", UnsetDate},
		},
		{
			name: "empty ldflags values count as unset",
			ld:   [3]string{"", "", ""},
			bi:   buildInfo("v1.0.0", "vcs.revision", rev, "vcs.time", "2026-02-03T04:05:06Z"),
			want: Info{"v1.0.0", "0123456", "2026-02-03T04:05:06Z"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.ld[0], tt.ld[1], tt.ld[2], tt.bi); got != tt.want {
				t.Errorf("Resolve() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
