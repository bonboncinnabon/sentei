package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

// The badge text is worktree.StatusBadge; the list only adds its color.
func TestStatusIndicator_RendersTheSharedBadge(t *testing.T) {
	for _, wt := range []git.Worktree{
		{},
		{HasUncommittedChanges: true},
		{HasUntrackedFiles: true},
		{IsLocked: true},
		{EnrichmentError: "git status: fatal: bad object HEAD"},
	} {
		if got, want := stripAnsi(statusIndicator(wt)), worktree.StatusBadge(wt); got != want {
			t.Errorf("statusIndicator(%+v) rendered %q, want %q", wt, got, want)
		}
	}
}

func TestViewLegend(t *testing.T) {
	m := NewModel(nil, nil, "")
	got := stripAnsi(m.viewLegend())

	for _, want := range []string{"[ok] clean", "[~] dirty", "[!] untracked", "[L] locked", "[P] protected"} {
		if !strings.Contains(got, want) {
			t.Errorf("viewLegend() = %q, want it to contain %q", got, want)
		}
	}
}

func keyMsg(k string) tea.KeyPressMsg {
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
}

func TestToggle_SkipsProtectedWorktree(t *testing.T) {
	wts := []git.Worktree{
		{Path: "/work/main", Branch: "refs/heads/main"},
		{Path: "/work/feature", Branch: "refs/heads/feature/x"},
	}
	m := NewModel(wts, nil, "/repo")
	m.remove.cursor = 0

	updated, _ := m.Update(keyMsg(" "))
	m = updated.(Model)

	if m.remove.selected["/work/main"] {
		t.Error("protected worktree should not be selectable via spacebar")
	}

	m.remove.cursor = 1
	updated, _ = m.Update(keyMsg(" "))
	m = updated.(Model)

	if !m.remove.selected["/work/feature"] {
		t.Error("non-protected worktree should be selectable via spacebar")
	}
}

func TestSelectAll_SkipsProtectedWorktrees(t *testing.T) {
	wts := []git.Worktree{
		{Path: "/work/main", Branch: "refs/heads/main"},
		{Path: "/work/feature", Branch: "refs/heads/feature/x"},
		{Path: "/work/bugfix", Branch: "refs/heads/bugfix/y"},
	}
	m := NewModel(wts, nil, "/repo")

	updated, _ := m.Update(keyMsg("a"))
	m = updated.(Model)

	if m.remove.selected["/work/main"] {
		t.Error("protected worktree should not be selected by select-all")
	}
	if !m.remove.selected["/work/feature"] {
		t.Error("non-protected worktree should be selected by select-all")
	}
	if !m.remove.selected["/work/bugfix"] {
		t.Error("non-protected worktree should be selected by select-all")
	}
	if len(m.remove.selected) != 2 {
		t.Errorf("expected 2 selected, got %d", len(m.remove.selected))
	}
}

func TestSelectAll_SkipsNonStandardDefaultBranch(t *testing.T) {
	wts := []git.Worktree{
		{Path: "/work/production", Branch: "refs/heads/production"},
		{Path: "/work/feature", Branch: "refs/heads/feature/x"},
	}
	m := NewModel(wts, nil, "/repo")
	m.remove.defaultBranch = "production" // set by loadWorktreeContext in production

	updated, _ := m.Update(keyMsg("a"))
	m = updated.(Model)

	if m.remove.selected["/work/production"] {
		t.Error("non-standard default branch worktree must not be selected by select-all")
	}
	if !m.remove.selected["/work/feature"] {
		t.Error("feature worktree should be selected by select-all")
	}
	if len(m.remove.selected) != 1 {
		t.Errorf("expected 1 selected, got %d", len(m.remove.selected))
	}
}

func TestSelectAll_WithFilter_SkipsProtected(t *testing.T) {
	now := time.Now()
	wts := []git.Worktree{
		{Path: "/work/main", Branch: "refs/heads/main", LastCommitDate: now},
		{Path: "/work/feat-a", Branch: "refs/heads/feature/a", LastCommitDate: now},
		{Path: "/work/feat-b", Branch: "refs/heads/feature/b", LastCommitDate: now},
		{Path: "/work/dev", Branch: "refs/heads/dev", LastCommitDate: now},
		{Path: "/work/bugfix", Branch: "refs/heads/bugfix/z", LastCommitDate: now},
	}
	m := NewModel(wts, nil, "/repo")

	m.remove.filterText = "feat"
	m.reindex()

	if len(m.remove.visibleIndices) != 2 {
		t.Fatalf("expected 2 visible after filter, got %d", len(m.remove.visibleIndices))
	}

	updated, _ := m.Update(keyMsg("a"))
	m = updated.(Model)

	if !m.remove.selected["/work/feat-a"] || !m.remove.selected["/work/feat-b"] {
		t.Error("visible non-protected worktrees should be selected")
	}
	if m.remove.selected["/work/main"] || m.remove.selected["/work/dev"] {
		t.Error("protected worktrees should not be selected even if hidden by filter")
	}
	if len(m.remove.selected) != 2 {
		t.Errorf("expected 2 selected, got %d", len(m.remove.selected))
	}
}

func TestDeselectAll_WithProtected(t *testing.T) {
	wts := []git.Worktree{
		{Path: "/work/main", Branch: "refs/heads/main"},
		{Path: "/work/feature", Branch: "refs/heads/feature/x"},
		{Path: "/work/bugfix", Branch: "refs/heads/bugfix/y"},
	}
	m := NewModel(wts, nil, "/repo")
	m.remove.selected["/work/feature"] = true
	m.remove.selected["/work/bugfix"] = true

	updated, _ := m.Update(keyMsg("a"))
	m = updated.(Model)

	if len(m.remove.selected) != 0 {
		t.Errorf("expected 0 selected after deselect-all, got %d", len(m.remove.selected))
	}
}

func TestViewStatusBar_FitsMinimumWidth(t *testing.T) {
	wts := []git.Worktree{{Path: "/work/feature", Branch: "refs/heads/feature/x"}}
	m := NewModel(wts, nil, "/repo")
	m.width = 80

	bar := m.viewStatusBar()
	for _, line := range strings.Split(bar, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("status bar line exceeds 80 cols (%d): %q", w, stripAnsi(line))
		}
	}
	plain := stripAnsi(bar)
	for _, want := range []string{"? details", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Errorf("status bar at 80 cols must include %q, got %q", want, plain)
		}
	}
}

func TestSortArrow_AgeDescribesDisplayedOrder(t *testing.T) {
	m := NewModel([]git.Worktree{{Path: "/a", Branch: "refs/heads/a"}}, nil, "/repo")
	m.width = 100
	m.view = listView

	// Default: age sort, date-ascending → displayed ages run largest-first.
	if view := stripAnsi(m.viewList()); !strings.Contains(view, "Age ▼") {
		t.Errorf("date-ascending shows descending ages; arrow must point down:\n%s", view)
	}
	m.remove.sortAscending = false
	if view := stripAnsi(m.viewList()); !strings.Contains(view, "Age ▲") {
		t.Errorf("date-descending shows ascending ages; arrow must point up:\n%s", view)
	}
}
