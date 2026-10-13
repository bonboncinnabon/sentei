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

// overflowingLines returns the lines of view wider than width, as text.
func overflowingLines(view string, width int) []string {
	var over []string
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > width {
			over = append(over, stripAnsi(line))
		}
	}
	return over
}

func narrowRemovalModel(width int) Model {
	m := NewModel([]git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/feature/extremely-long-branch-name-for-testing", HasUncommittedChanges: true},
		{Path: "/work/b", Branch: "refs/heads/chore/old-dependencies-cleanup", HasUntrackedFiles: true},
		{Path: "/work/c", Branch: "refs/heads/fix/c", IsLocked: true},
		{Path: "/work/d", Branch: "refs/heads/fix/d", HasUnpushedCommits: true},
		{Path: "/work/e", Branch: "refs/heads/fix/e", EnrichmentError: "git status: fatal: bad object HEAD"},
		{Path: "/work/f", Branch: "refs/heads/fix/f"},
	}, nil, "/repo")
	m.width = width
	for _, wt := range m.remove.worktrees {
		m.remove.selected[wt.Path] = true
	}
	return m
}

func TestList_FitsNarrowTerminals(t *testing.T) {
	for _, width := range []int{24, 32, 40, 48, 56, 60} {
		plain := narrowRemovalModel(width)

		labelled := narrowRemovalModel(width)
		labelled.remove.filterLabel = "stale > 30d"
		labelled.remove.filterText = "feature"
		labelled.reindex()

		filtering := narrowRemovalModel(width)
		filtering.remove.filterActive = true
		filtering.remove.filterInput.SetValue("a-long-filter-term-typed-by-the-user")

		for name, m := range map[string]Model{"plain": plain, "with filter labels": labelled, "filter input": filtering} {
			if over := overflowingLines(m.viewList(), width); len(over) > 0 {
				t.Errorf("list (%s) at %d columns has %d rows wider than the terminal:\n%s", name, width, len(over), strings.Join(over, "\n"))
			}
		}
	}
}

func TestConfirm_FitsNarrowTerminals(t *testing.T) {
	for _, width := range []int{24, 32, 40, 48, 56, 60} {
		m := narrowRemovalModel(width)
		view := m.viewConfirm()
		if over := overflowingLines(view, width); len(over) > 0 {
			t.Errorf("confirm at %d columns has %d rows wider than the terminal:\n%s", width, len(over), strings.Join(over, "\n"))
		}
		// Narrowing may shorten the rows, never drop what they warn about.
		for _, badge := range []string{"[~]", "[!]", "[L]", "[^]", "[?]", "[ok]"} {
			if !strings.Contains(stripAnsi(view), badge) {
				t.Errorf("confirm at %d columns lost the %s row", width, badge)
			}
		}
	}
}
