package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
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
			"/work/feature:[log -1 --format=%ci]":                         {Output: "2024-06-01 12:00:00 +0000"},
			"/work/feature:[log -1 --format=%s]":                          {Output: "Add feature X"},
			"/work/feature:[status --porcelain --untracked-files=normal]": {Output: " M file.go\n?? new.txt"},
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
			"/work/broken:[log -1 --format=%ci]": {Err: fmt.Errorf("git log: fatal: not a git repository")},
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
			"/work/broken:[log -1 --format=%ci]":                         {Output: "2024-01-01 00:00:00 +0000"},
			"/work/broken:[log -1 --format=%s]":                          {Output: "Some commit"},
			"/work/broken:[status --porcelain --untracked-files=normal]": {Err: fmt.Errorf("git status: permission denied")},
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
			"/work/normal:[log -1 --format=%ci]":                         {Output: "2024-03-15 09:00:00 +0000"},
			"/work/normal:[log -1 --format=%s]":                          {Output: "Normal commit"},
			"/work/normal:[status --porcelain --untracked-files=normal]": {Output: ""},
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
			"/work/ok:[log -1 --format=%ci]":                         {Output: "2024-03-15 09:00:00 +0000"},
			"/work/ok:[log -1 --format=%s]":                          {Output: "Good commit"},
			"/work/ok:[status --porcelain --untracked-files=normal]": {Output: ""},
			"/work/broken:[log -1 --format=%ci]":                     {Err: fmt.Errorf("directory missing")},
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
		"/work/feature:[log -1 --format=%ci]":                         {Output: "2024-06-01 12:00:00 +0000"},
		"/work/feature:[log -1 --format=%s]":                          {Output: "Add feature X"},
		"/work/feature:[status --porcelain --untracked-files=normal]": {Output: ""},
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
			"/work/feature:[log -1 --format=%ci]":                         {Output: "2024-06-01 12:00:00 +0000"},
			"/work/feature:[log -1 --format=%s]":                          {Output: "Add feature X"},
			"/work/feature:[status --porcelain --untracked-files=normal]": {Output: ""},
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
			"/work/feature:[log -1 --format=%ci]":                         {Output: "2024-06-01 12:00:00 +0000"},
			"/work/feature:[log -1 --format=%s]":                          {Output: "Add feature X"},
			"/work/feature:[status --porcelain --untracked-files=normal]": {Output: ""},
			"/work/feature:[rev-list --count HEAD --not --remotes]":       {Output: "0"},
		},
	}

	wt := &git.Worktree{Path: "/work/feature"}
	enrichWorktree(runner, wt, true)

	if wt.HasUnpushedCommits {
		t.Error("a branch fully contained in a remote must not be flagged unpushed even with no upstream tracking")
	}
}

// A rebase or amend rewrites a commit but keeps its author date. The age that
// --stale filters on and the list shows must follow the committer date, or a
// branch rebased today looks as old as its first draft.
func TestEnrichWorktrees_LastCommitDateIsCommitterDate(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(testtmp.HermeticGitEnv(),
			"GIT_AUTHOR_DATE=2020-01-01T00:00:00Z",
			"GIT_COMMITTER_DATE=2024-06-01T12:00:00Z",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "written in 2020, rebased in 2024")

	enriched := EnrichWorktrees(&git.GitRunner{}, []git.Worktree{{Path: dir}}, 1)

	if enriched[0].EnrichmentError != "" {
		t.Fatalf("enrichment failed: %s", enriched[0].EnrichmentError)
	}
	want := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	if !enriched[0].LastCommitDate.Equal(want) {
		t.Errorf("LastCommitDate = %v, want the committer date %v", enriched[0].LastCommitDate, want)
	}
}

// status.showUntrackedFiles=no makes a plain `git status --porcelain` omit
// untracked files. The at-risk gate reads that output, so a worktree holding
// an unsaved file would count as clean and be removed without --force.
func TestEnrichWorktrees_SeesUntrackedFilesDespiteUserConfig(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = testtmp.HermeticGitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial commit")
	run("config", "status.showUntrackedFiles", "no")
	if err := os.WriteFile(filepath.Join(dir, "draft.txt"), []byte("unsaved\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	enriched := EnrichWorktrees(&git.GitRunner{}, []git.Worktree{{Path: dir}}, 1)

	if enriched[0].EnrichmentError != "" {
		t.Fatalf("enrichment failed: %s", enriched[0].EnrichmentError)
	}
	if !enriched[0].HasUntrackedFiles {
		t.Error("the untracked file must be reported whatever the user's status.showUntrackedFiles says")
	}
	if !AtRisk(enriched[0]) {
		t.Error("a worktree with an untracked file is at risk")
	}
}
