package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func wheelDown() tea.MouseWheelMsg { return tea.MouseWheelMsg{Button: tea.MouseWheelDown} }
func wheelUp() tea.MouseWheelMsg   { return tea.MouseWheelMsg{Button: tea.MouseWheelUp} }

func wheelListModel() Model {
	wts := []git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/feature/a"},
		{Path: "/work/b", Branch: "refs/heads/feature/b"},
		{Path: "/work/c", Branch: "refs/heads/feature/c"},
	}
	m := NewModel(wts, nil, "/repo")
	m.height = 10
	return m
}

func TestListView_WheelDownMovesCursorDown(t *testing.T) {
	m := wheelListModel()
	m.remove.cursor = 1

	updated, _ := m.Update(wheelDown())
	if got := updated.(Model).remove.cursor; got != 2 {
		t.Errorf("cursor = %d after wheel down, want 2", got)
	}
}

func TestListView_WheelUpMovesCursorUp(t *testing.T) {
	m := wheelListModel()
	m.remove.cursor = 1

	updated, _ := m.Update(wheelUp())
	if got := updated.(Model).remove.cursor; got != 0 {
		t.Errorf("cursor = %d after wheel up, want 0", got)
	}
}

func TestListView_WheelUpAtTopBoundaryStays(t *testing.T) {
	m := wheelListModel()
	m.remove.cursor = 0

	updated, _ := m.Update(wheelUp())
	if got := updated.(Model).remove.cursor; got != 0 {
		t.Errorf("cursor = %d after wheel up at top, want 0", got)
	}
}

func TestListView_WheelDownAtBottomBoundaryStays(t *testing.T) {
	m := wheelListModel()
	m.remove.cursor = 2

	updated, _ := m.Update(wheelDown())
	if got := updated.(Model).remove.cursor; got != 2 {
		t.Errorf("cursor = %d after wheel down at bottom, want 2", got)
	}
}

func TestPortal_WheelScrollsViewportNotBackground(t *testing.T) {
	m := wheelListModel()
	m.remove.cursor = 1
	m.portal = m.portal.SetSize(80, 12)
	m.portal = m.portal.Open(portalHelp, "Help", strings.Repeat("line\n", 50))

	updated, _ := m.Update(wheelDown())
	model := updated.(Model)

	if model.portal.viewport.YOffset() == 0 {
		t.Error("wheel down should scroll the portal viewport")
	}
	if got := model.remove.cursor; got != 1 {
		t.Errorf("list cursor = %d, wheel must not reach the background view", got)
	}
}

// Regression: lipgloss v2 Style.Width includes border and padding (v1
// excluded the border), so a portal sized with v1 semantics wraps full-width
// content lines, leaving orphan fragments under the wrapped row.
func TestPortal_FullWidthLineDoesNotWrapInsideBox(t *testing.T) {
	var p DetailPortal
	p = p.SetSize(120, 30)
	long := strings.Repeat("x", p.contentWidth())
	p = p.Open(portalDetails, "T", long)

	bg := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", 120)+"\n", 30), "\n")
	out := stripANSI(p.View(bg))

	if !strings.Contains(out, long) {
		t.Errorf("a contentWidth-wide line must render unwrapped inside the portal box:\n%s", out)
	}
}

func TestIntegrationList_WheelMovesCursor(t *testing.T) {
	m := makeIntegrationModel()
	m.integ.cursor = 0

	updated, _ := m.updateIntegrationList(wheelDown())
	if got := updated.(Model).integ.cursor; got != 1 {
		t.Errorf("cursor = %d after wheel down, want 1", got)
	}

	updated, _ = updated.(Model).updateIntegrationList(wheelUp())
	if got := updated.(Model).integ.cursor; got != 0 {
		t.Errorf("cursor = %d after wheel up, want 0", got)
	}
}
