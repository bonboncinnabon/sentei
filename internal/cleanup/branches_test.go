package cleanup

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

const goneBranchCall = "/repo:[for-each-ref " + goneBranchFormat + " refs/heads]"

// refLine builds one for-each-ref line in goneBranchFormat.
func refLine(name, upstream, track, worktreePath string) string {
	return strings.Join([]string{name, upstream, track, worktreePath}, "\x1f")
}

func goneRef(name string) string {
	return refLine(name, "refs/remotes/origin/"+name, "[gone]", "")
}

func TestListGoneBranches(t *testing.T) {
	refs := strings.Join([]string{
		refLine("main", "refs/remotes/origin/main", "", "/repo/main"),
		refLine("local-only", "", "", ""),
		refLine("ahead", "refs/remotes/origin/ahead", "[ahead 1]", ""),
		goneRef("feature/old"),
		refLine("fix/in-wt", "refs/remotes/origin/fix/in-wt", "[gone]", "/repo/fix-in-wt"),
		refLine("pending", "refs/remotes/origin/pending", "", ""),
		goneRef("develop"),
		refLine("trunk", "refs/remotes/origin/trunk", "[gone]", "/repo/.bare"),
	}, "\n")
	runner := &mock.Runner{Responses: map[string]mock.Response{
		goneBranchCall:                      {Output: refs},
		"/repo:[symbolic-ref --short HEAD]": {Output: "trunk"},
	}}

	gone, kept, err := listGoneBranches(runner, "/repo", []string{"origin/pending"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := []string{"feature/old", "pending"}; !slices.Equal(gone, want) {
		t.Errorf("gone = %v, want %v", gone, want)
	}
	wantKept := []SkippedBranch{
		{Name: "fix/in-wt", Reason: SkipInWorktree},
		{Name: "develop", Reason: SkipProtected},
		{Name: "trunk", Reason: SkipProtected},
	}
	if !slices.Equal(kept, wantKept) {
		t.Errorf("kept = %v, want %v", kept, wantKept)
	}
}

func TestDeleteGoneBranches(t *testing.T) {
	tests := []struct {
		name           string
		refs           string
		extraResponses map[string]mock.Response
		dryRun         bool
		wantDeleted    []string
		wantSkipped    []SkippedBranch
	}{
		{
			name: "no gone branches",
			refs: refLine("main", "refs/remotes/origin/main", "", "/repo/main"),
		},
		{
			name: "deletes gone branches",
			refs: goneRef("feature/old") + "\n" + refLine("main", "refs/remotes/origin/main", "", "/repo/main"),
			extraResponses: map[string]mock.Response{
				"/repo:[branch -d feature/old]": {Output: "Deleted branch feature/old"},
			},
			wantDeleted: []string{"feature/old"},
		},
		{
			name: "skips worktree-checkout branches",
			refs: refLine("fix/in-wt", "refs/remotes/origin/fix/in-wt", "[gone]", "/path/to/wt") + "\n" + goneRef("feature/gone"),
			extraResponses: map[string]mock.Response{
				"/repo:[branch -d feature/gone]": {Output: "Deleted branch feature/gone"},
			},
			wantDeleted: []string{"feature/gone"},
			wantSkipped: []SkippedBranch{{Name: "fix/in-wt", Reason: SkipInWorktree}},
		},
		{
			name: "skips a branch git refuses as unmerged",
			refs: goneRef("feature/unmerged"),
			extraResponses: map[string]mock.Response{
				"/repo:[branch -d feature/unmerged]":                         {Err: fmt.Errorf("error: branch not fully merged")},
				"/repo:[rev-parse --abbrev-ref feature/unmerged@{upstream}]": {Err: fmt.Errorf("no upstream")},
				"/repo:[merge-base --is-ancestor feature/unmerged HEAD]":     {Err: fmt.Errorf("exit status 1")},
			},
			wantSkipped: []SkippedBranch{{Name: "feature/unmerged", Reason: SkipUnmerged}},
		},
		{
			name: "dry run names the branches without deleting",
			refs: goneRef("feature/gone"),
			extraResponses: map[string]mock.Response{
				"/repo:[rev-parse --abbrev-ref feature/gone@{upstream}]": {Err: fmt.Errorf("no upstream")},
				"/repo:[merge-base --is-ancestor feature/gone HEAD]":     {Output: ""},
			},
			dryRun:      true,
			wantDeleted: []string{"feature/gone"},
		},
		{
			name: "dry run announces the skip of an unmerged branch",
			refs: goneRef("feature/unmerged"),
			extraResponses: map[string]mock.Response{
				"/repo:[rev-parse --abbrev-ref feature/unmerged@{upstream}]": {Err: fmt.Errorf("no upstream")},
				"/repo:[merge-base --is-ancestor feature/unmerged HEAD]":     {Err: fmt.Errorf("exit status 1")},
			},
			dryRun:      true,
			wantSkipped: []SkippedBranch{{Name: "feature/unmerged", Reason: SkipUnmerged}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := map[string]mock.Response{
				goneBranchCall:                      {Output: tt.refs},
				"/repo:[symbolic-ref --short HEAD]": {Output: "main"},
			}
			for k, v := range tt.extraResponses {
				responses[k] = v
			}

			runner := &mock.Runner{Responses: responses}
			opts := Options{DryRun: tt.dryRun}
			events := collectEvents(t)

			result, err := DeleteGoneBranches(runner, "/repo", nil, opts, events.Emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(result.Deleted, tt.wantDeleted) {
				t.Errorf("Deleted = %v, want %v", result.Deleted, tt.wantDeleted)
			}
			if !slices.Equal(result.Skipped, tt.wantSkipped) {
				t.Errorf("Skipped = %v, want %v", result.Skipped, tt.wantSkipped)
			}
		})
	}
}

// One failed deletion must not hide the deletions that worked, and must not
// be filed under "not fully merged" when the branch is merged.
func TestDeleteGoneBranches_FailureKeepsPartialResult(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		goneBranchCall:                                     {Output: goneRef("locked") + "\n" + goneRef("fine")},
		"/repo:[symbolic-ref --short HEAD]":                {Output: "main"},
		"/repo:[branch -d locked]":                         {Err: fmt.Errorf("cannot lock ref 'refs/heads/locked'")},
		"/repo:[rev-parse --abbrev-ref locked@{upstream}]": {Err: fmt.Errorf("no upstream")},
		"/repo:[merge-base --is-ancestor locked HEAD]":     {Output: ""},
		"/repo:[branch -d fine]":                           {Output: "Deleted branch fine"},
	}}

	result, err := DeleteGoneBranches(runner, "/repo", nil, Options{Mode: ModeSafe}, collectEvents(t).Emit)

	if err == nil || !strings.Contains(err.Error(), "locked") || !strings.Contains(err.Error(), "cannot lock ref") {
		t.Errorf("err = %v, want it to name the branch and carry git's reason", err)
	}
	if want := []string{"fine"}; !slices.Equal(result.Deleted, want) {
		t.Errorf("Deleted = %v, want %v", result.Deleted, want)
	}
	if len(result.Skipped) != 0 {
		t.Errorf("Skipped = %v, want none: the failure was not about unmerged work", result.Skipped)
	}
}

func TestCleanNonWorktreeBranches(t *testing.T) {
	worktreeList := "worktree /repo\nbare\n\nworktree /repo/main\nHEAD abc123\nbranch refs/heads/main\n\nworktree /repo/feature-x\nHEAD def456\nbranch refs/heads/feature-x"

	branchList := "main\nfeature-x\nfeature/old\nfix/stale\ndevelop"

	tests := []struct {
		name          string
		mode          Mode
		force         bool
		dryRun        bool
		wantDeleted   []string
		wantRemaining int
	}{
		{
			name:          "safe mode only counts",
			mode:          ModeSafe,
			wantRemaining: 2,
		},
		{
			name:        "aggressive deletes non-worktree branches",
			mode:        ModeAggressive,
			wantDeleted: []string{"feature/old", "fix/stale"},
		},
		{
			name:          "aggressive dry run announces only what -d deletes",
			mode:          ModeAggressive,
			dryRun:        true,
			wantDeleted:   []string{"feature/old"},
			wantRemaining: 1,
		},
		{
			name:        "aggressive dry run with force announces every candidate",
			mode:        ModeAggressive,
			force:       true,
			dryRun:      true,
			wantDeleted: []string{"feature/old", "fix/stale"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{Responses: map[string]mock.Response{
				"/repo:[worktree list --porcelain]":                            {Output: worktreeList},
				"/repo:[for-each-ref --format=%(refname:lstrip=2) refs/heads]": {Output: branchList},
				"/repo:[symbolic-ref --short HEAD]":                            {Output: "main"},
				"/repo:[branch -d feature/old]":                                {Output: ""},
				"/repo:[branch -d fix/stale]":                                  {Output: ""},
				"/repo:[branch -D feature/old]":                                {Output: ""},
				"/repo:[branch -D fix/stale]":                                  {Output: ""},
				// Only the dry run asks: feature/old is merged, fix/stale is not.
				"/repo:[rev-parse --abbrev-ref feature/old@{upstream}]": {Err: fmt.Errorf("no upstream")},
				"/repo:[merge-base --is-ancestor feature/old HEAD]":     {Output: ""},
				"/repo:[rev-parse --abbrev-ref fix/stale@{upstream}]":   {Err: fmt.Errorf("no upstream")},
				"/repo:[merge-base --is-ancestor fix/stale HEAD]":       {Err: fmt.Errorf("exit status 1")},
			}}
			opts := Options{Mode: tt.mode, Force: tt.force, DryRun: tt.dryRun}
			events := collectEvents(t)

			result, err := CleanNonWorktreeBranches(runner, "/repo", nil, nil, opts, events.Emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(result.Deleted, tt.wantDeleted) {
				t.Errorf("Deleted = %v, want %v", result.Deleted, tt.wantDeleted)
			}
			if result.Remaining != tt.wantRemaining {
				t.Errorf("Remaining = %d, want %d", result.Remaining, tt.wantRemaining)
			}
		})
	}
}

func TestCleanNonWorktreeBranches_UnmergedIsSkippedAndRemains(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[worktree list --porcelain]":                            {Output: "worktree /repo\nbare"},
		"/repo:[for-each-ref --format=%(refname:lstrip=2) refs/heads]": {Output: "main\nwip\ndone"},
		"/repo:[symbolic-ref --short HEAD]":                            {Output: "main"},
		"/repo:[branch -d wip]":                                        {Err: fmt.Errorf("error: the branch 'wip' is not fully merged")},
		"/repo:[rev-parse --abbrev-ref wip@{upstream}]":                {Err: fmt.Errorf("no upstream")},
		"/repo:[merge-base --is-ancestor wip HEAD]":                    {Err: fmt.Errorf("exit status 1")},
		"/repo:[branch -d done]":                                       {Output: ""},
	}}

	result, err := CleanNonWorktreeBranches(runner, "/repo", nil, nil, Options{Mode: ModeAggressive}, collectEvents(t).Emit)
	if err != nil {
		t.Fatalf("an unmerged branch is a skip, not an error: %v", err)
	}
	if want := []string{"done"}; !slices.Equal(result.Deleted, want) {
		t.Errorf("Deleted = %v, want %v", result.Deleted, want)
	}
	if want := []SkippedBranch{{Name: "wip", Reason: SkipUnmerged}}; !slices.Equal(result.Skipped, want) {
		t.Errorf("Skipped = %v, want %v", result.Skipped, want)
	}
	if result.Remaining != 1 {
		t.Errorf("Remaining = %d, want 1", result.Remaining)
	}
}

func TestDeleteGoneBranches_ForceGatedByMode(t *testing.T) {
	// An unmerged gone branch: -d fails (not fully merged), -D would succeed.
	refs := goneRef("feature/unmerged")

	t.Run("safe+force keeps unmerged (uses -d, never -D)", func(t *testing.T) {
		runner := &mock.Runner{Responses: map[string]mock.Response{
			goneBranchCall:                                               {Output: refs},
			"/repo:[symbolic-ref --short HEAD]":                          {Output: "main"},
			"/repo:[branch -d feature/unmerged]":                         {Err: fmt.Errorf("error: branch not fully merged")},
			"/repo:[rev-parse --abbrev-ref feature/unmerged@{upstream}]": {Err: fmt.Errorf("no upstream")},
			"/repo:[merge-base --is-ancestor feature/unmerged HEAD]":     {Err: fmt.Errorf("exit status 1")},
			// -D is mocked to SUCCEED: if the code wrongly used it in safe mode,
			// the branch would be deleted and this test would fail.
			"/repo:[branch -D feature/unmerged]": {Output: "Deleted"},
		}}
		events := collectEvents(t)
		result, err := DeleteGoneBranches(runner, "/repo", nil, Options{Mode: ModeSafe, Force: true}, events.Emit)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Deleted) != 0 || len(result.Skipped) != 1 {
			t.Errorf("safe+force must keep an unmerged branch (use -d): deleted=%v skipped=%v", result.Deleted, result.Skipped)
		}
	})

	t.Run("aggressive+force force-deletes unmerged (uses -D)", func(t *testing.T) {
		runner := &mock.Runner{Responses: map[string]mock.Response{
			goneBranchCall:                       {Output: refs},
			"/repo:[symbolic-ref --short HEAD]":  {Output: "main"},
			"/repo:[branch -D feature/unmerged]": {Output: "Deleted branch feature/unmerged"},
		}}
		events := collectEvents(t)
		result, err := DeleteGoneBranches(runner, "/repo", nil, Options{Mode: ModeAggressive, Force: true}, events.Emit)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Deleted) != 1 {
			t.Errorf("aggressive+force must force-delete the unmerged branch (use -D): deleted=%v", result.Deleted)
		}
	})
}

// A dry run prunes nothing, so a branch whose upstream is about to be pruned
// still resolves it. git will judge the branch against HEAD by the time it
// deletes, and the prediction must do the same.
func TestBranchDeletableByGit_StaleUpstreamCountsAsMissing(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[rev-parse --abbrev-ref pushed@{upstream}]":      {Output: "origin/pushed"},
		"/repo:[merge-base --is-ancestor pushed origin/pushed]": {Output: ""},
		"/repo:[merge-base --is-ancestor pushed HEAD]":          {Err: fmt.Errorf("exit status 1")},
	}}

	if !branchDeletableByGit(runner, "/repo", "pushed", nil) {
		t.Error("with a live upstream the branch is merged into it and deletable")
	}
	if branchDeletableByGit(runner, "/repo", "pushed", []string{"origin/pushed"}) {
		t.Error("with the upstream about to be pruned the branch is judged against HEAD, where it is unmerged")
	}
}

// Branches the gone-upstream step deletes are not there for the next step.
func TestCleanNonWorktreeBranches_LeavesOutWhatTheGoneStepDeleted(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[worktree list --porcelain]":                            {Output: "worktree /repo\nbare"},
		"/repo:[for-each-ref --format=%(refname:lstrip=2) refs/heads]": {Output: "main\ngone/merged\nextra"},
		"/repo:[symbolic-ref --short HEAD]":                            {Output: "main"},
	}}

	result, err := CleanNonWorktreeBranches(runner, "/repo", nil, []string{"gone/merged"}, Options{Mode: ModeSafe, DryRun: true}, collectEvents(t).Emit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Remaining != 1 {
		t.Errorf("Remaining = %d, want 1: only extra is left once gone/merged is deleted", result.Remaining)
	}
}
