package cleanup

import (
	"fmt"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestDeleteGoneBranches(t *testing.T) {
	tests := []struct {
		name           string
		branchVV       string
		extraResponses map[string]mock.Response
		dryRun         bool
		wantDeleted    int
		wantSkipped    int
	}{
		{
			name:        "no gone branches",
			branchVV:    "  main abc123 [origin/main] latest commit",
			wantDeleted: 0,
		},
		{
			name:     "deletes gone branches",
			branchVV: "  feature/old abc123 [origin/feature/old: gone] old commit\n  main def456 [origin/main] latest",
			extraResponses: map[string]mock.Response{
				"/repo:[branch -d feature/old]": {Output: "Deleted branch feature/old"},
			},
			wantDeleted: 1,
		},
		{
			name:     "skips worktree-checkout branches",
			branchVV: "+ fix/in-wt abc123 (/path/to/wt) [origin/fix/in-wt: gone] commit\n  feature/gone def456 [origin/feature/gone: gone] commit",
			extraResponses: map[string]mock.Response{
				"/repo:[branch -d feature/gone]": {Output: "Deleted branch feature/gone"},
			},
			wantDeleted: 1,
			wantSkipped: 1,
		},
		{
			name:     "skips unmerged on delete failure",
			branchVV: "  feature/unmerged abc123 [origin/feature/unmerged: gone] commit",
			extraResponses: map[string]mock.Response{
				"/repo:[branch -d feature/unmerged]": {Err: fmt.Errorf("error: branch not fully merged")},
			},
			wantDeleted: 0,
			wantSkipped: 1,
		},
		{
			name:        "dry run counts without deleting",
			branchVV:    "  feature/gone abc123 [origin/feature/gone: gone] commit",
			dryRun:      true,
			wantDeleted: 1,
			wantSkipped: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := map[string]mock.Response{
				"/repo:[branch -vv]": {Output: tt.branchVV},
			}
			for k, v := range tt.extraResponses {
				responses[k] = v
			}

			runner := &mock.Runner{Responses: responses}
			opts := Options{DryRun: tt.dryRun}
			events := collectEvents(t)

			result, err := DeleteGoneBranches(runner, "/repo", opts, events.Emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Deleted != tt.wantDeleted {
				t.Errorf("Deleted = %d, want %d", result.Deleted, tt.wantDeleted)
			}
			if len(result.Skipped) != tt.wantSkipped {
				t.Errorf("Skipped = %d, want %d", len(result.Skipped), tt.wantSkipped)
			}
		})
	}
}

func TestParseGoneBranches(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantGone         int
		wantWorktreeGone int
	}{
		{name: "empty output", input: ""},
		{name: "no gone branches", input: "  main abc123 [origin/main] latest"},
		{name: "standard gone branch", input: "  feature/old abc123 [origin/feature/old: gone] commit", wantGone: 1},
		{name: "worktree-checkout gone branch", input: "+ fix/wt abc123 (/path) [origin/fix/wt: gone] commit", wantWorktreeGone: 1},
		{name: "current branch with gone upstream", input: "* feature/current abc123 [origin/feature/current: gone] commit", wantWorktreeGone: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gone, wtGone := parseGoneBranches(tt.input)
			if len(gone) != tt.wantGone {
				t.Errorf("gone = %d, want %d", len(gone), tt.wantGone)
			}
			if len(wtGone) != tt.wantWorktreeGone {
				t.Errorf("worktreeGone = %d, want %d", len(wtGone), tt.wantWorktreeGone)
			}
		})
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
		wantDeleted   int
		wantRemaining int
		wantSkipped   int
	}{
		{
			name:          "safe mode only counts",
			mode:          ModeSafe,
			wantDeleted:   0,
			wantRemaining: 2,
		},
		{
			name:        "aggressive deletes non-worktree branches",
			mode:        ModeAggressive,
			wantDeleted: 2,
		},
		{
			name:        "aggressive dry run",
			mode:        ModeAggressive,
			dryRun:      true,
			wantDeleted: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{Responses: map[string]mock.Response{
				"/repo:[worktree list --porcelain]":        {Output: worktreeList},
				"/repo:[branch --format=%(refname:short)]": {Output: branchList},
				"/repo:[branch -d feature/old]":            {Output: ""},
				"/repo:[branch -d fix/stale]":              {Output: ""},
				"/repo:[branch -D feature/old]":            {Output: ""},
				"/repo:[branch -D fix/stale]":              {Output: ""},
			}}
			opts := Options{Mode: tt.mode, Force: tt.force, DryRun: tt.dryRun}
			events := collectEvents(t)

			result, err := CleanNonWorktreeBranches(runner, "/repo", opts, events.Emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Deleted != tt.wantDeleted {
				t.Errorf("Deleted = %d, want %d", result.Deleted, tt.wantDeleted)
			}
			if result.Remaining != tt.wantRemaining {
				t.Errorf("Remaining = %d, want %d", result.Remaining, tt.wantRemaining)
			}
		})
	}
}

func TestDeleteGoneBranches_ForceGatedByMode(t *testing.T) {
	// An unmerged gone branch: -d fails (not fully merged), -D would succeed.
	const branchVV = "  feature/unmerged abc123 [origin/feature/unmerged: gone] commit"

	t.Run("safe+force keeps unmerged (uses -d, never -D)", func(t *testing.T) {
		runner := &mock.Runner{Responses: map[string]mock.Response{
			"/repo:[branch -vv]":                 {Output: branchVV},
			"/repo:[branch -d feature/unmerged]": {Err: fmt.Errorf("error: branch not fully merged")},
			// -D is mocked to SUCCEED: if the code wrongly used it in safe mode,
			// the branch would be deleted and this test would fail.
			"/repo:[branch -D feature/unmerged]": {Output: "Deleted"},
		}}
		events := collectEvents(t)
		result, err := DeleteGoneBranches(runner, "/repo", Options{Mode: ModeSafe, Force: true}, events.Emit)
		if err != nil {
			t.Fatal(err)
		}
		if result.Deleted != 0 || len(result.Skipped) != 1 {
			t.Errorf("safe+force must keep an unmerged branch (use -d): deleted=%d skipped=%d", result.Deleted, len(result.Skipped))
		}
	})

	t.Run("aggressive+force force-deletes unmerged (uses -D)", func(t *testing.T) {
		runner := &mock.Runner{Responses: map[string]mock.Response{
			"/repo:[branch -vv]":                 {Output: branchVV},
			"/repo:[branch -D feature/unmerged]": {Output: "Deleted branch feature/unmerged"},
		}}
		events := collectEvents(t)
		result, err := DeleteGoneBranches(runner, "/repo", Options{Mode: ModeAggressive, Force: true}, events.Emit)
		if err != nil {
			t.Fatal(err)
		}
		if result.Deleted != 1 {
			t.Errorf("aggressive+force must force-delete the unmerged branch (use -D): deleted=%d", result.Deleted)
		}
	})
}
