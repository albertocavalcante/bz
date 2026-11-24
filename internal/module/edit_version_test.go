package module

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const versionRule = `must match RELEASE[-PRERELEASE][+BUILD] using only A-Z, a-z, 0-9, '.', '-' and '+'`

// hostileVersions are values a compromised registry or a typo could supply.
var hostileVersions = []struct {
	name    string
	version string
	want    string
}{
	{"quote breaking", `9.9.9", repo_name = "evil`, `refusing to write module "a": invalid module version "9.9.9\", repo_name = \"evil": ` + versionRule},
	{"single quote", `1.0'`, `refusing to write module "a": invalid module version "1.0'": ` + versionRule},
	{"newline", "1.0\nbazel_dep(name = \"evil\", version = \"1\")", `refusing to write module "a": invalid module version "1.0\nbazel_dep(name = \"evil\", version = \"1\")": ` + versionRule},
	{"comment marker", "1.0 # x", `refusing to write module "a": invalid module version "1.0 # x": ` + versionRule},
	{"backslash", `1.0\`, `refusing to write module "a": invalid module version "1.0\\": ` + versionRule},
	{"spaces", "1 0", `refusing to write module "a": invalid module version "1 0": ` + versionRule},
	{"control byte", "1.0\x01", `refusing to write module "a": invalid module version "1.0\x01": ` + versionRule},
	{"empty", "", `refusing to write module "a": invalid module version: version is empty (overridden modules have no registry version)`},
}

func TestSetDepVersion_RefusesInvalidVersion(t *testing.T) {
	t.Parallel()

	const src = "module(name = \"m\", version = \"1.0\")\nbazel_dep(name = \"a\", version = \"1.0.0\")\n"
	for _, tc := range hostileVersions {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := []byte(src)
			got, changed, err := SetDepVersion(in, "a", tc.version)
			require.EqualError(t, err, tc.want)
			assert.False(t, changed)
			var invalid *InvalidDepError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, "a", invalid.Module)
			assert.Nil(t, got)
			assert.Equal(t, src, string(in))
		})
	}
}

func TestAppendDeps_RefusesInvalidDep(t *testing.T) {
	t.Parallel()

	const src = "module(name = \"m\", version = \"1.0\")\n"
	tests := []struct {
		name string
		dep  DepSpec
		want string
	}{
		{"quote breaking version", DepSpec{Name: "a", Version: hostileVersions[0].version}, hostileVersions[0].want},
		{"newline version", DepSpec{Name: "a", Version: hostileVersions[2].version}, hostileVersions[2].want},
		{"empty version", DepSpec{Name: "a"}, hostileVersions[7].want},
		{
			"quote breaking name",
			DepSpec{Name: `a", repo_name = "evil`, Version: "1.0.0"},
			`refusing to write module "a\", repo_name = \"evil": invalid module name "a\", repo_name = \"evil": ` +
				`must start with a lowercase letter, end with a lowercase letter or digit, and contain only a-z, 0-9, '.', '-' and '_'`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := []byte(src)
			got, err := AppendDeps(in, []DepSpec{{Name: "ok", Version: "1.0.0"}, tc.dep})
			require.EqualError(t, err, tc.want)
			assert.Nil(t, got)
			assert.Equal(t, src, string(in))
		})
	}
}
