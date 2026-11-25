package module

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanVersionEdits_GroupsFilesAndDeduplicatesRequests(t *testing.T) {
	const root = "MODULE.bazel"
	const included = "d/x.MODULE.bazel"
	const rootOriginal = "bazel_dep(name = \"a\", version = \"1\")\nbazel_dep(name = \"c\", version = \"1\")\n"
	const includedOriginal = "# keep\r\nbazel_dep(name = 'b', version = '1')\r\nbazel_dep(name = \"b\", version = \"1\")\r\nbazel_dep(name = \"d\")"
	snapshot := map[string][]byte{root: []byte(rootOriginal), included: []byte(includedOriginal)}

	plan, err := PlanVersionEdits(snapshot, []VersionEdit{
		{Rel: root, Name: "a", Version: "2"},
		{Rel: included, Name: "b", Current: "1", Version: "2"},
		{Rel: root, Name: "c", Version: "2"},
		{Rel: included, Name: "b", Version: "3"},
		{Rel: included, Name: "d", Version: "2"},
	})
	require.NoError(t, err)

	assert.Equal(t, []int{0, 2, 1}, plan.Applied)
	assert.Equal(t, []int{4}, plan.Versionless)
	require.Len(t, plan.Files, 2)
	assert.Equal(t, root, plan.Files[0].Rel)
	assert.Equal(t, rootOriginal, string(plan.Files[0].Original))
	assert.Equal(t, "bazel_dep(name = \"a\", version = \"2\")\nbazel_dep(name = \"c\", version = \"2\")\n", string(plan.Files[0].Updated))
	assert.Equal(t, included, plan.Files[1].Rel)
	assert.Equal(t, includedOriginal, string(plan.Files[1].Original))
	assert.Equal(t, "# keep\r\nbazel_dep(name = 'b', version = '2')\r\nbazel_dep(name = \"b\", version = \"2\")\r\nbazel_dep(name = \"d\")", string(plan.Files[1].Updated))

	assert.Equal(t, rootOriginal, string(snapshot[root]))
	assert.Equal(t, includedOriginal, string(snapshot[included]))
	snapshot[included][0] = '!'
	assert.Equal(t, includedOriginal, string(plan.Files[1].Original), "the plan retains the bytes it was computed from")
}

func TestPlanVersionEdits_FailureDiscardsEarlierEdits(t *testing.T) {
	const root = "MODULE.bazel"
	const included = "d/x.MODULE.bazel"
	snapshot := map[string][]byte{
		root:     []byte("bazel_dep(name = \"a\", version = \"1\")\n"),
		included: []byte("bazel_dep(name = \"b\", version = \"2\")\n"),
	}

	for _, tc := range []struct {
		name    string
		request VersionEdit
		wantErr string
	}{
		{
			name:    "unchanged version",
			request: VersionEdit{Rel: included, Name: "b", Version: "2"},
			wantErr: "failed to update b: version 2 would not change MODULE.bazel text",
		},
		{
			name:    "missing captured file",
			request: VersionEdit{Rel: "missing.MODULE.bazel", Name: "b", Version: "3"},
			wantErr: "missing captured content for missing.MODULE.bazel",
		},
		{
			name:    "missing declaration",
			request: VersionEdit{Rel: included, Name: "c", Version: "3"},
			wantErr: "failed to update c: bazel_dep not found",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := PlanVersionEdits(snapshot, []VersionEdit{
				{Rel: root, Name: "a", Version: "2"},
				tc.request,
			})
			require.ErrorContains(t, err, tc.wantErr)
			assert.Equal(t, VersionPlan{}, plan)
			assert.Equal(t, "bazel_dep(name = \"a\", version = \"1\")\n", string(snapshot[root]))
		})
	}
}
