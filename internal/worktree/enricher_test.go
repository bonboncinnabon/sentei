package worktree

import (
	"fmt"
	"testing"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestParseStatusPorcelain(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantUncommitted bool
		wantUntracked   bool
	}{
		{
			name:            "empty output (clean)",
			input:           "",
			wantUncommitted: false,
			wantUntracked:   false,
		},
		{
			name:            "whitespace only (clean)",
			input:           "  \n  ",
			wantUncommitted: false,
			wantUntracked:   false,
		},
		{
			name:            "uncommitted changes only (modified)",
			input:           " M internal/git/worktree.go",
			wantUncommitted: true,
			wantUntracked:   false,
		},
		{
			name:            "staged addition",
			input:           "A  newfile.go",
			wantUncommitted: true,
			wantUntracked:   false,
		},
		{
			name:            "untracked files only",
			input:           "?? untracked.txt\n?? another.txt",
			wantUncommitted: false,
			wantUntracked:   true,
		},
		{
			name:            "both uncommitted and untracked",
			input:           " M changed.go\n?? newfile.txt",
			wantUncommitted: true,
			wantUntracked:   true,
		},
		{
			name:            "deleted file",
			input:           " D removed.go",
			wantUncommitted: true,
			wantUntracked:   false,
		},
		{
			name:            "renamed file",
			input:           "R  old.go -> new.go",
			wantUncommitted: true,
			wantUntracked:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUncommitted, gotUntracked := ParseStatusPorcelain(tt.input)
			if gotUncommitted != tt.wantUncommitted {
				t.Errorf("hasUncommitted = %v, want %v", gotUncommitted, tt.wantUncommitted)
			}
			if gotUntracked != tt.wantUntracked {
				t.Errorf("hasUntracked = %v, want %v", gotUntracked, tt.wantUntracked)
			}
		})
	}
}

func TestParseCommitDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "valid date",
			input:   "2024-01-15 10:30:00 -0500",
			want:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("", -5*3600)),
			wantErr: false,
		},
		{
			name:    "empty output (orphan branch)",
			input:   "",
			want:    time.Time{},
			wantErr: false,
		},
		{
			name:    "whitespace only",
			input:   "  \n  ",
			want:    time.Time{},
			wantErr: false,
		},
		{
			name:    "malformed date",
			input:   "not-a-date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCommitDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnrichWorktree_Success(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/work/feature:[log -1 --format=%ai]": {Output: "2024-06-01 12:00:00 +0000"},
			"/work/feature:[log -1 --format=%s]":  {Output: "Add feature X"},
			"/work/feature:[status --porcelain]":  {Output: " M file.go\n?? new.txt"},
		},
	}

	wt := &git.Worktree{Path: "/work/feature"}
	enrichWorktree(runner, wt, false)

	if wt.EnrichmentError != "" {
		t.Fatalf("unexpected error: %s", wt.EnrichmentError)
	}
	if !wt.IsEnriched {
		t.Error("expected IsEnriched=true")
	}
	if wt.LastCommitSubject != "Add feature X" {
		t.Errorf("LastCommitSubject = %q, want %q", wt.LastCommitSubject, "Add feature X")
	}
	if !wt.HasUncommittedChanges {
		t.Error("expected HasUncommittedChanges=true")
	}
	if !wt.HasUntrackedFiles {
		t.Error("expected HasUntrackedFiles=true")
	}
	expectedDate := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	if !wt.LastCommitDate.Equal(expectedDate) {
		t.Errorf("LastCommitDate = %v, want %v", wt.LastCommitDate, expectedDate)
	}
}

func TestEnrichWorktree_LogCommandFails(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/work/broken:[log -1 --format=%ai]": {Err: fmt.Errorf("git log: fatal: not a git repository")},
		},
	}

	wt := &git.Worktree{Path: "/work/broken"}
	enrichWorktree(runner, wt, false)

	if wt.EnrichmentError == "" {
		t.Error("expected EnrichmentError to be set")
	}
	if wt.IsEnriched {
		t.Error("expected IsEnriched=false")
	}
}

func TestEnrichWorktree_StatusCommandFails(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/work/broken:[log -1 --format=%ai]": {Output: "2024-01-01 00:00:00 +0000"},
			"/work/broken:[log -1 --format=%s]":  {Output: "Some commit"},
			"/work/broken:[status --porcelain]":  {Err: fmt.Errorf("git status: permission denied")},
		},
	}

	wt := &git.Worktree{Path: "/work/broken"}
	enrichWorktree(runner, wt, false)

	if wt.EnrichmentError == "" {
		t.Error("expected EnrichmentError to be set")
	}
	if wt.IsEnriched {
		t.Error("expected IsEnriched=false")
	}
}

func TestEnrichWorktrees_MixedSlice(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/work/normal:[log -1 --format=%ai]": {Output: "2024-03-15 09:00:00 +0000"},
			"/work/normal:[log -1 --format=%s]":  {Output: "Normal commit"},
			"/work/normal:[status --porcelain]":  {Output: ""},
		},
	}

	worktrees := []git.Worktree{
		{Path: "/repo", IsBare: true},
		{Path: "/work/normal"},
		{Path: "/work/gone", IsPrunable: true, PruneReason: "directory not found"},
	}

	result := EnrichWorktrees(runner, worktrees, 5)

	if result[0].IsEnriched {
		t.Error("bare entry should not be enriched")
	}
	if !result[1].IsEnriched {
		t.Error("normal entry should be enriched")
	}
	if result[1].LastCommitSubject != "Normal commit" {
		t.Errorf("LastCommitSubject = %q, want %q", result[1].LastCommitSubject, "Normal commit")
	}
	if result[2].IsEnriched {
		t.Error("prunable entry should not be enriched")
	}
}

func TestEnrichWorktrees_PartialFailure(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/work/ok:[log -1 --format=%ai]":     {Output: "2024-03-15 09:00:00 +0000"},
			"/work/ok:[log -1 --format=%s]":      {Output: "Good commit"},
			"/work/ok:[status --porcelain]":      {Output: ""},
			"/work/broken:[log -1 --format=%ai]": {Err: fmt.Errorf("directory missing")},
		},
	}

	worktrees := []git.Worktree{
		{Path: "/work/ok"},
		{Path: "/work/broken"},
	}

	result := EnrichWorktrees(runner, worktrees, 5)

	if !result[0].IsEnriched {
		t.Error("first worktree should be enriched")
	}
	if result[0].EnrichmentError != "" {
		t.Errorf("first worktree should have no error, got: %s", result[0].EnrichmentError)
	}
	if result[1].IsEnriched {
		t.Error("second worktree should not be enriched")
	}
	if result[1].EnrichmentError == "" {
		t.Error("second worktree should have enrichment error")
	}
}

func TestParseAheadCount(t *testing.T) {
	cases := []struct {
		output string
		want   int
	}{
		{"0\n", 0},
		{"3", 3},
		{"  12  ", 12},
		{"garbage", 1}, // unparsable errs toward "ahead" so the gate engages
		{"", 1},
	}
	for _, tc := range cases {
		if got := ParseAheadCount(tc.output); got != tc.want {
			t.Errorf("ParseAheadCount(%q) = %d, want %d", tc.output, got, tc.want)
		}
	}
}

func TestEnrichWorktree_UnpushedDetection(t *testing.T) {
	base := map[string]mock.Response{
		"/work/feature:[log -1 --format=%ai]": {Output: "2024-06-01 12:00:00 +0000"},
		"/work/feature:[log -1 --format=%s]":  {Output: "Add feature X"},
		"/work/feature:[status --porcelain]":  {Output: ""},
	}
	cases := []struct {
		name         string
		revList      *mock.Response
		wantUnpushed bool
	}{
		{"all commits reachable from a remote", &mock.Response{Output: "0"}, false},
		{"commits reachable from no remote", &mock.Response{Output: "2"}, true},
		{"probe failure errs toward caution", nil, true}, // rev-list error -> assume unpushed
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			responses := make(map[string]mock.Response, len(base)+1)
			for k, v := range base {
				responses[k] = v
			}
			if tc.revList != nil {
				responses["/work/feature:[rev-list --count HEAD --not --remotes]"] = *tc.revList
			}
			runner := &mock.Runner{Responses: responses}

			wt := &git.Worktree{Path: "/work/feature"}
			enrichWorktree(runner, wt, true)

			if !wt.IsEnriched {
				t.Fatalf("enrichment failed: %s", wt.EnrichmentError)
			}
			if wt.HasUnpushedCommits != tc.wantUnpushed {
				t.Errorf("HasUnpushedCommits = %v, want %v", wt.HasUnpushedCommits, tc.wantUnpushed)
			}
		})
	}
}

func TestEnrichWorktree_NoRemotesNeverUnpushed(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/work/feature:[log -1 --format=%ai]": {Output: "2024-06-01 12:00:00 +0000"},
			"/work/feature:[log -1 --format=%s]":  {Output: "Add feature X"},
			"/work/feature:[status --porcelain]":  {Output: ""},
		},
	}

	wt := &git.Worktree{Path: "/work/feature"}
	enrichWorktree(runner, wt, false)

	if wt.HasUnpushedCommits {
		t.Error("a repo without remotes must not flag branches as unpushed")
	}
}

// A branch whose upstream was deleted after merge still has all its commits on
// a remote (via the merge into the default branch), so it must not be flagged
// unpushed. The old @{upstream}..HEAD probe errored on the missing upstream and
// false-flagged it; HEAD --not --remotes counts commits on no remote (0 here).
func TestEnrichWorktree_GoneUpstreamMergedNotUnpushed(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/work/feature:[log -1 --format=%ai]":                   {Output: "2024-06-01 12:00:00 +0000"},
			"/work/feature:[log -1 --format=%s]":                    {Output: "Add feature X"},
			"/work/feature:[status --porcelain]":                    {Output: ""},
			"/work/feature:[rev-list --count HEAD --not --remotes]": {Output: "0"},
		},
	}

	wt := &git.Worktree{Path: "/work/feature"}
	enrichWorktree(runner, wt, true)

	if wt.HasUnpushedCommits {
		t.Error("a branch fully contained in a remote must not be flagged unpushed even with no upstream tracking")
	}
}
