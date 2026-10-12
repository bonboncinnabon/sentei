package cleanup

import (
	"fmt"
	"slices"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestOrphanedBranchSections(t *testing.T) {
	tests := []struct {
		name     string
		branches string
		keys     string
		want     []string
	}{
		{
			name:     "no branch sections",
			branches: "main",
			keys:     "core.bare\nremote.origin.url",
		},
		{
			name:     "every section has its branch",
			branches: "main\nfeature/old-work",
			keys:     "branch.main.remote\nbranch.main.merge\nbranch.feature/old-work.remote",
		},
		{
			name:     "a section is reported once however many keys it has",
			branches: "main",
			keys:     "branch.main.remote\nbranch.gone.remote\nbranch.gone.merge\nbranch.gone.github-pr-owner-number",
			want:     []string{"gone"},
		},
		{
			name:     "names keep their dots and slashes",
			branches: "feature/deep/nested.v2",
			keys:     "branch.feature/deep/nested.v2.merge\nbranch.fix/JIRA-1234.v1.remote\nbranch.2026-03-01-migration.merge",
			want:     []string{"fix/JIRA-1234.v1", "2026-03-01-migration"},
		},
		{
			name:     "settings of the branch section itself are not a branch",
			branches: "main",
			keys:     "branch.autosetupmerge\nbranch.sort",
		},
		{
			name:     "no branches at all",
			branches: "",
			keys:     "branch.main.remote\nbranch.other.remote",
			want:     []string{"main", "other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{Responses: map[string]mock.Response{
				"/repo:[branch --format=%(refname:lstrip=2)]": {Output: tt.branches},
				"/repo:[config --local --name-only --list]":   {Output: tt.keys},
			}}

			got, err := orphanedBranchSections(runner, "/repo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("orphans = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPurgeOrphanedBranchConfigs_StopsAtFirstFailure(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[branch --format=%(refname:lstrip=2)]":      {Output: "main"},
		"/repo:[config --local --name-only --list]":        {Output: "branch.a.remote\nbranch.b.remote"},
		"/repo:[config --local --remove-section branch.a]": {Output: ""},
		"/repo:[config --local --remove-section branch.b]": {Err: fmt.Errorf("could not lock config file")},
	}}

	result, err := PurgeOrphanedBranchConfigs(runner, "/repo", Options{}, collectEvents(t).Emit)

	if err == nil {
		t.Fatal("expected the failed removal to be returned")
	}
	if result.Removed != 1 {
		t.Errorf("Removed = %d, want 1: section a was removed before b failed", result.Removed)
	}
}
