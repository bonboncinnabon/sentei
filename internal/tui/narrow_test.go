package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func narrowListModel(width int) Model {
	wts := []git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/feature/extremely-long-branch-name-for-testing", LastCommitSubject: "A very long commit subject line that overflows"},
		{Path: "/work/b", Branch: "refs/heads/chore/old-dependencies-cleanup", LastCommitSubject: "Short"},
	}
	m := NewModel(wts, nil, "/repo")
	m.width = width
	m.height = 12
	return m
}

func TestList_OneLineRowsAtNarrowWidth(t *testing.T) {
	m := narrowListModel(60)
	out := stripAnsi(m.viewList())

	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("line %d exceeds 60 cols (%d): %q", i, w, line)
		}
	}
	if !strings.Contains(out, "…") {
		t.Error("long branch names must truncate with …")
	}
	if strings.Contains(out, "...") {
		t.Error("ASCII ... must not appear; the idiom is …")
	}
	if !strings.Contains(out, "selected") {
		t.Error("status bar must remain visible at narrow width")
	}
	if !strings.Contains(out, "[ok]") {
		t.Error("legend must remain visible at narrow width")
	}
}

func TestList_ColumnPriorityDropsDetail(t *testing.T) {
	wide := stripAnsi(narrowListModel(100).viewList())
	if !strings.Contains(wide, "Subject") || !strings.Contains(wide, "Age") {
		t.Fatal("wide layout must show Subject and Age")
	}

	mid := stripAnsi(narrowListModel(60).viewList())
	if strings.Contains(mid, "Subject") {
		t.Error("Subject column must be dropped below 72 cols")
	}
	if !strings.Contains(mid, "Age") {
		t.Error("Age column must survive at 60 cols")
	}

	tiny := stripAnsi(narrowListModel(50).viewList())
	if strings.Contains(tiny, "Age") || strings.Contains(tiny, "Subject") {
		t.Error("Age and Subject must be dropped below 56 cols")
	}
	if !strings.Contains(tiny, "Branch") {
		t.Error("Branch column must always survive")
	}
}

func TestPortal_FitsNarrowTerminal(t *testing.T) {
	m := narrowListModel(60)
	m.portal = m.portal.SetSize(60, 18)
	m.portal = m.portal.Open(portalDetails, "Worktree details", strings.Repeat("x", 200))

	bg := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", 60)+"\n", 18), "\n")
	for i, line := range strings.Split(stripAnsi(m.portal.View(bg)), "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("portal line %d exceeds 60 cols (%d): %q", i, w, line)
		}
	}
}

func TestCreateView_HeaderTruncates(t *testing.T) {
	m := createBranchModel()
	m.width = 60
	m.repoPath = "/very/long/playground/path/that/overflows/sixty/columns/easily/repo.git"

	for i, line := range strings.Split(stripAnsi(m.viewCreateBranch()), "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("create view line %d exceeds 60 cols (%d): %q", i, w, line)
		}
	}
}
