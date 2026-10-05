package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

// Golden chrome pinning: exact rendered output of the stable views, ANSI
// included, so chrome regressions (spacing, vocabulary, styling) fail loudly.
// Regenerate intentionally with `go test ./internal/tui/ -run TestGolden -update`.
//
// Fixtures must be deterministic: fixed width, settled (non-working) states,
// and commit dates as short now-relative offsets so age buckets are stable.

func goldenWorktrees() []git.Worktree {
	return []git.Worktree{
		{Path: "/work/main", Branch: "refs/heads/main", HEAD: "aaaaaaaaaaaa", LastCommitDate: time.Now().Add(-5 * time.Minute), LastCommitSubject: "Initial commit"},
		{Path: "/work/clean", Branch: "refs/heads/feature/clean", HEAD: "bbbbbbbbbbbb", LastCommitDate: time.Now().Add(-10 * time.Minute), LastCommitSubject: "Add feature"},
		{Path: "/work/dirty", Branch: "refs/heads/feature/dirty", HEAD: "cccccccccccc", LastCommitDate: time.Now().Add(-30 * time.Minute), LastCommitSubject: "WIP", HasUncommittedChanges: true},
		{Path: "/work/untracked", Branch: "refs/heads/experiment/x", HEAD: "dddddddddddd", LastCommitDate: time.Now().Add(-45 * time.Minute), LastCommitSubject: "Spike", HasUntrackedFiles: true},
	}
}

func goldenModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(goldenWorktrees(), nil, "/repo")
	m.width = 80
	m.height = 30
	m.remove.defaultBranch = "main"
	return m
}

func TestGoldenListView(t *testing.T) {
	m := goldenModel(t)
	m.view = listView
	golden.RequireEqual(t, []byte(m.viewList()))
}

func TestGoldenConfirmView(t *testing.T) {
	m := goldenModel(t)
	m.view = confirmView
	m.remove.selected = map[string]bool{"/work/clean": true, "/work/dirty": true, "/work/untracked": true}
	golden.RequireEqual(t, []byte(m.viewConfirm()))
}

func TestGoldenSummaryView(t *testing.T) {
	m := goldenModel(t)
	m.view = summaryView
	m.remove.run = newRemovalRun(nil)
	m.remove.run.result = worktree.DeletionResult{SuccessCount: 3}
	m.remove.run.cleanupResult = &cleanup.Result{StaleRefsRemoved: 2}
	m.remove.milestone = 10
	golden.RequireEqual(t, []byte(m.viewSummary()))
}

func TestGoldenCleanupResultView(t *testing.T) {
	m := goldenModel(t)
	m.view = cleanupResultView
	m.cleanupResult = &cleanup.Result{
		StaleRefsRemoved:    2,
		GoneBranchesDeleted: 1,
	}
	golden.RequireEqual(t, []byte(m.viewCleanupResult()))
}

func TestGoldenCreateInputView(t *testing.T) {
	m := goldenModel(t)
	m.view = createBranchView
	m.create.branchInput.SetValue("feat/demo")
	golden.RequireEqual(t, []byte(m.viewCreateBranch()))
}
