package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func confirmModel(clean, untracked int) Model {
	var worktrees []git.Worktree
	for i := range clean {
		name := fmt.Sprintf("clean-%02d", i)
		worktrees = append(worktrees, git.Worktree{Path: "/repo/" + name, Branch: "refs/heads/" + name})
	}
	for i := range untracked {
		name := fmt.Sprintf("dirty-%02d", i)
		worktrees = append(worktrees, git.Worktree{Path: "/repo/" + name, Branch: "refs/heads/" + name, HasUntrackedFiles: true})
	}
	m := makeRemoveModel(worktrees)
	for _, wt := range worktrees {
		m.remove.selected[wt.Path] = true
	}
	m.view = confirmView
	return m
}

// The confirm view had no height bound: with more rows than the terminal,
// the at-risk rows, the "will be lost" totals and the y/n footer were all
// clipped off the bottom.
func TestViewConfirm_FitsTerminalKeepingRiskRowsAndFooter(t *testing.T) {
	m := confirmModel(20, 10)
	m.width, m.windowHeight = 80, 24

	out := m.viewConfirm()

	if lines := strings.Count(out, "\n"); lines > m.windowHeight {
		t.Errorf("confirm view is %d lines on a %d-row terminal", lines, m.windowHeight)
	}
	for _, want := range []string{
		"dirty-00",
		"dirty-09",
		"10 worktrees with untracked files that will be lost",
		"more",
		viewFooterDanger(m.width, confirmFooter),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("confirm view is missing %q:\n%s", want, out)
		}
	}
}

func TestViewConfirm_HidesAtRiskRowsOnlyWithACount(t *testing.T) {
	m := confirmModel(0, 30)
	m.width, m.windowHeight = 80, 24

	out := m.viewConfirm()

	if lines := strings.Count(out, "\n"); lines > m.windowHeight {
		t.Errorf("confirm view is %d lines on a %d-row terminal", lines, m.windowHeight)
	}
	if !strings.Contains(out, "of them at risk") {
		t.Errorf("hidden at-risk rows must be counted on the summary line:\n%s", out)
	}
}

func TestViewConfirm_ShowsEveryRowWhenItFits(t *testing.T) {
	m := confirmModel(2, 1)
	m.width, m.windowHeight = 80, 40

	out := m.viewConfirm()

	for _, want := range []string{"clean-00", "clean-01", "dirty-00"} {
		if !strings.Contains(out, want) {
			t.Errorf("confirm view is missing row %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "more") {
		t.Errorf("nothing should be summarised when every row fits:\n%s", out)
	}
}
