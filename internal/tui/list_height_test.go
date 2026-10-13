package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// tallListModel is a list of count worktrees named wt-00, wt-01, ... sized by
// a real window-size message, the way the runtime sizes it.
func tallListModel(t *testing.T, count, width, height int) Model {
	t.Helper()
	wts := make([]git.Worktree, count)
	for i := range wts {
		name := fmt.Sprintf("wt-%02d", i)
		wts[i] = git.Worktree{Path: "/work/" + name, Branch: "refs/heads/" + name}
	}
	m := NewModel(wts, nil, "/repo")
	m.remove.sortField = SortByBranch
	m.reindex()
	return resizeList(t, m, width, height)
}

// squeeze collapses runs of whitespace so column padding does not matter.
func squeeze(s string) string { return strings.Join(strings.Fields(s), " ") }

func resizeList(t *testing.T, m Model, width, height int) Model {
	t.Helper()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updated.(Model)
}

// assertListFits fails when the list view is taller than the terminal or has
// lost the rows a user steers by: the cursor, the selection count, the legend.
func assertListFits(t *testing.T, m Model, height int) {
	t.Helper()
	view := stripAnsi(m.viewList())
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("list view is %d rows on a %d-row terminal:\n%s", got, height, view)
	}
	for _, want := range []string{"▸", "selected", "[ok]"} {
		if !strings.Contains(view, want) {
			t.Errorf("list view on %d rows lost %q:\n%s", height, want, view)
		}
	}
}

func TestList_FitsTheTerminalHeight(t *testing.T) {
	for _, height := range []int{12, 16, 24, 40} {
		m := tallListModel(t, 30, 80, height)
		assertListFits(t, m, height)

		// The cursor stays on screen all the way down the list.
		for range 29 {
			updated, _ := m.Update(keyRune('j'))
			m = updated.(Model)
			assertListFits(t, m, height)
		}
		if view := stripAnsi(m.viewList()); !strings.Contains(squeeze(view), "▸ [ ] [ok] wt-29") {
			t.Errorf("height %d: cursor row wt-29 is not visible at the bottom:\n%s", height, view)
		}
	}
}

func TestList_ResizeKeepsTheCursorVisibleAndTheWindowFull(t *testing.T) {
	m := tallListModel(t, 30, 80, 50)
	for range 29 {
		updated, _ := m.Update(keyRune('j'))
		m = updated.(Model)
	}

	// Shrinking must scroll the window down to the cursor.
	m = resizeList(t, m, 80, 20)
	assertListFits(t, m, 20)
	if view := stripAnsi(m.viewList()); !strings.Contains(squeeze(view), "▸ [ ] [ok] wt-29") {
		t.Errorf("cursor row wt-29 scrolled off screen after shrinking:\n%s", view)
	}

	// Growing must not leave blank rows below a short tail.
	m = resizeList(t, m, 80, 50)
	if view := stripAnsi(m.viewList()); !strings.Contains(view, "wt-00") {
		t.Errorf("a terminal tall enough for every row still hides wt-00 after growing:\n%s", view)
	}
}
