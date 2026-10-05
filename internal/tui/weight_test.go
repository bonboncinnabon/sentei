package tui

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestWorktreeLabel_DetachedUsesShortHash(t *testing.T) {
	wt := git.Worktree{Path: "/work/detached-head", HEAD: "e635cdfa8afe577ac4345997dba1bab0ca223810", IsDetached: true}
	if got := worktreeLabel(wt); got != "e635cdf" {
		t.Errorf("detached label = %q, want short hash e635cdf (must match the list)", got)
	}
}

func TestConfirm_CleanUsesBadgeVocabulary(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/feature/a"},
	}, nil, "/repo")
	m.remove.selected = map[string]bool{"/work/a": true}
	m.width = 80

	view := stripAnsi(m.viewConfirm())
	if !strings.Contains(view, "[ok]") {
		t.Errorf("clean rows must use the list's badge vocabulary:\n%s", view)
	}
}
func TestStatusBar_DeleteHintNeedsSelection(t *testing.T) {
	wts := []git.Worktree{{Path: "/work/a", Branch: "refs/heads/feature/a"}}
	m := NewModel(wts, nil, "/repo")
	m.width = 80

	if got := stripAnsi(m.viewStatusBar()); strings.Contains(got, "enter delete") {
		t.Errorf("delete hint must hide at 0 selected: %q", got)
	}
	m.remove.selected["/work/a"] = true
	if got := stripAnsi(m.viewStatusBar()); !strings.Contains(got, "enter delete") {
		t.Errorf("delete hint must show with a selection: %q", got)
	}
}

func TestFilterPrompt_KeepsChromeMargin(t *testing.T) {
	wts := []git.Worktree{{Path: "/work/a", Branch: "refs/heads/feature/a"}}
	m := NewModel(wts, nil, "/repo")
	m.width = 80
	m.remove.filterActive = true

	line := stripAnsi(m.viewStatusOrFilter())
	if !strings.HasPrefix(line, "  ") {
		t.Errorf("filter prompt must keep the 2-space margin: %q", line)
	}
}

func TestDangerFooter_WeightsDestructiveKey(t *testing.T) {
	out := viewFooterDanger(80, confirmFooter)
	if !strings.Contains(out, "y delete") {
		t.Fatalf("danger footer must carry the destructive hint: %q", out)
	}
	plain := stripAnsi(out)
	if !strings.Contains(plain, "y delete · n go back") {
		t.Errorf("danger footer format: %q", plain)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("danger footer must carry styling")
	}
}
