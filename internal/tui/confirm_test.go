package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/playground"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func TestViewConfirm_CleanWorktrees(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/feature-a"},
		{Path: "/work/b", Branch: "refs/heads/feature-b"},
	}, nil, "/repo")
	m.remove.selected["/work/a"] = true
	m.remove.selected["/work/b"] = true
	m.view = confirmView

	output := stripAnsi(m.viewConfirm())

	if !strings.Contains(output, "delete 2 worktrees") {
		t.Error("should mention count of worktrees")
	}
	if !strings.Contains(output, "feature-a") {
		t.Error("should list feature-a")
	}
	if !strings.Contains(output, "[ok]") {
		t.Error("should show the [ok] badge for clean worktrees")
	}
	if strings.Contains(output, "[ok]  clean") || strings.Contains(output, "] clean") {
		t.Error("clean rows carry no note text; the badge says it")
	}
	if strings.Contains(output, "⚠") {
		t.Error("should not show warnings for clean worktrees")
	}
}

func TestViewConfirm_DirtyWorktree(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/dirty", Branch: "refs/heads/dirty-branch", HasUncommittedChanges: true},
	}, nil, "/repo")
	m.remove.selected["/work/dirty"] = true
	m.view = confirmView

	output := stripAnsi(m.viewConfirm())

	if !strings.Contains(output, "uncommitted changes — will be lost") {
		t.Error("should warn about uncommitted changes")
	}
	if !strings.Contains(output, "⚠") {
		t.Error("should show a warning line for dirty worktrees")
	}
}

func TestViewConfirm_LockedWorktree(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/locked", Branch: "refs/heads/locked-branch", IsLocked: true},
	}, nil, "/repo")
	m.remove.selected["/work/locked"] = true
	m.view = confirmView

	output := stripAnsi(m.viewConfirm())

	if !strings.Contains(output, "locked — will force-remove") {
		t.Error("should warn about locked worktree")
	}
	if !strings.Contains(output, "force-remove") {
		t.Error("should mention force-removal")
	}
}

func TestViewConfirm_UntrackedFiles(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/untracked", Branch: "refs/heads/untracked-branch", HasUntrackedFiles: true},
	}, nil, "/repo")
	m.remove.selected["/work/untracked"] = true
	m.view = confirmView

	output := stripAnsi(m.viewConfirm())

	if !strings.Contains(output, "untracked files") {
		t.Error("should warn about untracked files")
	}
}

func TestPlayground_DeleteAll_IncludesLockedWorktree(t *testing.T) {
	repoPath, cleanup, err := playground.Setup()
	if err != nil {
		t.Fatalf("playground setup: %v", err)
	}
	defer cleanup()

	runner := &git.GitRunner{}
	worktrees, err := git.ListWorktrees(runner, repoPath)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	worktrees = worktree.EnrichWorktrees(runner, worktrees, 5)

	// Verify there's exactly one locked worktree
	var lockedCount int
	var lockedPath string
	for _, wt := range worktrees {
		if wt.IsLocked {
			lockedCount++
			lockedPath = wt.Path
		}
	}
	if lockedCount != 1 {
		t.Fatalf("expected 1 locked worktree, got %d", lockedCount)
	}

	m := NewModel(worktrees, runner, repoPath)
	// Select all non-bare, non-protected worktrees (including locked)
	for _, wt := range worktrees {
		if !wt.IsBare && !git.IsProtectedBranch(wt.Branch) {
			m.remove.selected[wt.Path] = true
		}
	}
	runRemoval(t, m)

	// After deletion + prune, re-list worktrees
	remaining, err := git.ListWorktrees(runner, repoPath)
	if err != nil {
		t.Fatalf("ListWorktrees after delete: %v", err)
	}

	for _, wt := range remaining {
		if wt.Path == lockedPath {
			t.Errorf("locked worktree %s should have been removed but still appears in git worktree list", lockedPath)
		}
		if wt.IsLocked {
			t.Errorf("no locked worktrees should remain, found: %s", wt.Path)
		}
	}
}

func gateModel(wts []git.Worktree) Model {
	m := NewModel(wts, nil, "/repo")
	m.width, m.height = 90, 28
	m.view = listView
	m.remove.selected = map[string]bool{}
	for _, wt := range wts {
		m.remove.selected[wt.Path] = true
	}
	m.reindex()
	return m
}

func TestRemovalGate_CleanPushedSkipsConfirmation(t *testing.T) {
	m := gateModel([]git.Worktree{
		{Path: "/w/a", Branch: "refs/heads/a"},
		{Path: "/w/b", Branch: "refs/heads/b"},
	})

	updated, cmd := m.updateList(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != progressView {
		t.Errorf("clean+pushed selection must skip the gate, got view %d", model.view)
	}
	if cmd == nil {
		t.Error("expected the deletion pipeline to start")
	}
	if got := len(model.remove.run.targets); got != 2 {
		t.Errorf("run targets = %d, want 2", got)
	}
}

func TestRemovalGate_AtRiskTriggersConfirmation(t *testing.T) {
	cases := []struct {
		name string
		wt   git.Worktree
	}{
		{"dirty", git.Worktree{Path: "/w/x", Branch: "refs/heads/x", HasUncommittedChanges: true}},
		{"untracked", git.Worktree{Path: "/w/x", Branch: "refs/heads/x", HasUntrackedFiles: true}},
		{"unpushed", git.Worktree{Path: "/w/x", Branch: "refs/heads/x", HasUnpushedCommits: true}},
		{"locked", git.Worktree{Path: "/w/x", Branch: "refs/heads/x", IsLocked: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := gateModel([]git.Worktree{
				{Path: "/w/clean", Branch: "refs/heads/clean"},
				tc.wt,
			})

			updated, _ := m.updateList(tea.KeyPressMsg{Code: tea.KeyEnter})
			model := updated.(Model)

			if model.view != confirmView {
				t.Errorf("%s selection must stop at the gate, got view %d", tc.name, model.view)
			}
		})
	}
}

func TestViewConfirm_UnpushedWarning(t *testing.T) {
	m := gateModel([]git.Worktree{
		{Path: "/w/x", Branch: "refs/heads/feature/x", HasUnpushedCommits: true},
	})
	m.view = confirmView

	view := stripANSI(m.viewConfirm())
	for _, want := range []string{
		"commits not on any remote",
		"⚠ 1 worktree with commits not pushed to any remote",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
}

func TestViewConfirm_ColumnsAlign(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/chore/old-deps"},
		{Path: "/work/b", Branch: "refs/heads/experiment/abandoned", HasUntrackedFiles: true},
		{Path: "/work/c", Branch: "refs/heads/x", HasUncommittedChanges: true},
	}, nil, "/repo")
	m.remove.selected = map[string]bool{"/work/a": true, "/work/b": true, "/work/c": true}
	m.width = 100

	view := stripAnsi(m.viewConfirm())
	var nameCols, noteCols []int
	for _, line := range strings.Split(view, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if !strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "[ok]  clean") {
			continue
		}
		if i := strings.Index(line, "]"); i >= 0 {
			rest := line[i+1:]
			nameCols = append(nameCols, i+1+(len(rest)-len(strings.TrimLeft(rest, " "))))
		}
		for _, note := range []string{"untracked files", "uncommitted changes"} {
			if j := strings.Index(line, note); j >= 0 {
				noteCols = append(noteCols, j)
			}
		}
	}
	if len(nameCols) != 3 {
		t.Fatalf("expected 3 badge rows, got %d:\n%s", len(nameCols), view)
	}
	for _, c := range nameCols[1:] {
		if c != nameCols[0] {
			t.Errorf("names must start at one column: %v\n%s", nameCols, view)
		}
	}
	if len(noteCols) == 2 && noteCols[0] != noteCols[1] {
		t.Errorf("risk notes must start at one column: %v\n%s", noteCols, view)
	}
}
