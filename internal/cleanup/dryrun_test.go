package cleanup

import (
	"errors"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

// dryRunMock wires the git responses DryRun's scan needs: a gone-upstream
// branch (feature/gone), a worktree-occupied branch (main), a non-worktree
// branch with a live upstream (feature/extra) and no origin remote.
func dryRunMock() *mock.Runner {
	return &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[rev-parse --git-dir]":       {Output: "/repo/.bare"},
		"/repo:[remote]":                    {Output: ""},
		"/repo:[symbolic-ref --short HEAD]": {Output: "main"},
		goneBranchCall: {Output: goneRef("feature/gone") + "\n" +
			refLine("main", "refs/remotes/origin/main", "", "/repo/main") + "\n" +
			refLine("feature/extra", "refs/remotes/origin/feature/extra", "", "")},
		"/repo:[worktree list --porcelain]":           {Output: "worktree /repo\nbare\n\nworktree /repo/main\nHEAD def456\nbranch refs/heads/main\n"},
		"/repo:[branch --format=%(refname:lstrip=2)]": {Output: "feature/gone\nfeature/extra\nmain"},
		"/repo:[config --local --name-only --list]":   {Output: "core.bare\nbranch.main.remote"},
		"/repo:[for-each-ref --format=%(refname:short)\x1f%(committerdate:iso8601-strict)\x1f%(subject) refs/heads/]": {Output: "feature/gone\x1f2026-02-10T19:00:00+05:30\x1fold work\nfeature/extra\x1f2026-06-01T10:00:00+05:30\x1fwip\nmain\x1f2026-06-10T12:00:00+05:30\x1flatest"},
		// feature/extra is merged into its live upstream. feature/gone has no
		// upstream ref left, so git judges it against HEAD, where it is merged.
		"/repo:[rev-parse --abbrev-ref feature/extra@{upstream}]":             {Output: "origin/feature/extra"},
		"/repo:[merge-base --is-ancestor feature/extra origin/feature/extra]": {Output: ""},
		"/repo:[rev-parse --abbrev-ref feature/gone@{upstream}]":              {Err: errors.New("no such ref")},
		"/repo:[merge-base --is-ancestor feature/gone HEAD]":                  {Output: ""},
	}}
}

func TestDryRun_CollectsBothModes(t *testing.T) {
	runner := dryRunMock()

	result, err := DryRun(runner, "/repo")
	if err != nil {
		t.Fatalf("DryRun() error: %v", err)
	}

	if len(result.GoneBranches) != 1 || result.GoneBranches[0] != "feature/gone" {
		t.Errorf("GoneBranches = %v, want [feature/gone]", result.GoneBranches)
	}
	// feature/gone goes in safe mode already, so the only branch aggressive
	// mode adds is feature/extra; main is protected and in a worktree.
	if len(result.AggressiveBranches) != 1 {
		t.Fatalf("AggressiveBranches = %v, want only feature/extra", result.AggressiveBranches)
	}
	extra := result.AggressiveBranches[0]
	if extra.Name != "feature/extra" || extra.LastCommitSubject != "wip" || extra.LastCommitDate.IsZero() {
		t.Errorf("expected metadata on aggressive branches, got %+v", extra)
	}
	if !extra.Merged {
		t.Error("feature/extra is merged into its upstream and should be marked Merged")
	}
}

// A gone branch git would refuse to delete is not promised by the safe
// preview. It stays an aggressive candidate, marked unmerged.
func TestDryRun_UnmergedGoneBranchIsNotPromised(t *testing.T) {
	runner := dryRunMock()
	runner.Responses["/repo:[merge-base --is-ancestor feature/gone HEAD]"] = mock.Response{Err: errors.New("exit status 1")}

	result, err := DryRun(runner, "/repo")
	if err != nil {
		t.Fatalf("DryRun() error: %v", err)
	}
	if len(result.GoneBranches) != 0 {
		t.Errorf("GoneBranches = %v, want none: git branch -d refuses feature/gone", result.GoneBranches)
	}
	if len(result.AggressiveBranches) != 2 || result.AggressiveBranches[0].Name != "feature/gone" || result.AggressiveBranches[0].Merged {
		t.Errorf("AggressiveBranches = %+v, want feature/gone (unmerged) and feature/extra", result.AggressiveBranches)
	}
}

func TestBranchDeletableByGit(t *testing.T) {
	merged := mock.Response{}                             // exit 0 -> ancestor of base
	notMerged := mock.Response{Err: errors.New("exit 1")} // non-zero -> not an ancestor
	tests := []struct {
		name     string
		upstream mock.Response // rev-parse --abbrev-ref b@{upstream}
		base     string        // base the is-ancestor check targets
		ancestor mock.Response // merge-base --is-ancestor b base
		want     bool
	}{
		{"merged into live upstream", mock.Response{Output: "origin/b"}, "origin/b", merged, true},
		{"merged into HEAD but not upstream", mock.Response{Output: "origin/b"}, "origin/b", notMerged, false},
		{"no upstream, merged into HEAD", mock.Response{Err: errors.New("no upstream")}, "HEAD", merged, true},
		{"no upstream, not merged into HEAD", mock.Response{Err: errors.New("no upstream")}, "HEAD", notMerged, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{Responses: map[string]mock.Response{
				"/repo:[rev-parse --abbrev-ref b@{upstream}]":        tt.upstream,
				"/repo:[merge-base --is-ancestor b " + tt.base + "]": tt.ancestor,
			}}
			if got := branchDeletableByGit(runner, "/repo", "b", nil); got != tt.want {
				t.Errorf("branchDeletableByGit = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDryRun_MutatesNothing(t *testing.T) {
	runner := dryRunMock()

	if _, err := DryRun(runner, "/repo"); err != nil {
		t.Fatalf("DryRun() error: %v", err)
	}

	assertNoMutatingCall(t, runner)
}

// A probe that fails must surface: its count stays zero, and without the
// error the preview would read that as "nothing to do".
func TestDryRun_CollectsProbeErrors(t *testing.T) {
	runner := dryRunMock()
	runner.Responses["/repo:[remote]"] = mock.Response{Err: errors.New("remote unreachable")}
	runner.Responses["/repo:[config --local --name-only --list]"] = mock.Response{Err: errors.New("bad config line 3")}

	result, err := DryRun(runner, "/repo")
	if err != nil {
		t.Fatalf("a failed probe must not abort the scan: %v", err)
	}
	for _, step := range []string{"prune-refs", "orphaned-configs"} {
		if stepError(result.Errors, step) == nil {
			t.Errorf("missing %s error, got %v", step, result.Errors)
		}
	}
	if len(result.GoneBranches) != 1 {
		t.Errorf("the other probes must still report, GoneBranches = %v", result.GoneBranches)
	}
}

func TestDryRunResult_HasWork(t *testing.T) {
	cases := []struct {
		name           string
		result         DryRunResult
		wantSafe       bool
		wantAggressive bool
	}{
		{"empty", DryRunResult{}, false, false},
		{"stale refs only", DryRunResult{StaleRefs: 2}, true, false},
		{"gone branches only", DryRunResult{GoneBranches: []string{"a"}}, true, false},
		{"orphaned configs only", DryRunResult{OrphanedConfigs: 1}, true, false},
		{"prunable worktrees only", DryRunResult{PrunableWorktrees: 1}, true, false},
		{"aggressive only", DryRunResult{AggressiveBranches: []BranchInfo{{Name: "x"}}}, false, true},
		{"both", DryRunResult{StaleRefs: 1, AggressiveBranches: []BranchInfo{{Name: "x"}}}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.result.SafeHasWork(); got != tc.wantSafe {
				t.Errorf("SafeHasWork() = %v, want %v", got, tc.wantSafe)
			}
			if got := tc.result.AggressiveHasWork(); got != tc.wantAggressive {
				t.Errorf("AggressiveHasWork() = %v, want %v", got, tc.wantAggressive)
			}
		})
	}
}

func TestDryRun_ErrorWhenNotARepository(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{}}
	if _, err := DryRun(runner, "/repo"); err == nil {
		t.Error("expected an error when the path is not a repository")
	}
}
