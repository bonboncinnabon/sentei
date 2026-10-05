package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func TestViewSummary_BoundsEveryLineAndOffersFullWidthOmissions(t *testing.T) {
	for _, width := range []int{20, 40, 80} {
		m := NewModel(nil, nil, "/repo")
		m.view = summaryView
		m.width, m.windowHeight, m.height = width, 40, 34
		m.portal = m.portal.SetSize(width, 40)
		fullPath := "/repo/界/" + strings.Repeat("very-long-worktree-name-", 5)
		fullError := strings.Repeat("錯誤🙂 failure detail ", 8)
		m.remove.run.result.FailureCount = 1
		m.remove.run.result.Outcomes = []worktree.WorktreeOutcome{{Path: fullPath, Error: errors.New(fullError)}}

		view := m.viewSummary()
		for row, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d row %d rendered %d cells: %q", width, row+1, got, stripANSI(line))
			}
		}
		if !strings.Contains(stripANSI(view), "? details") {
			t.Fatalf("width %d summary did not advertise width-omitted detail:\n%s", width, stripANSI(view))
		}
		title, detail := m.detailContent()
		compact := compactProgressDetailText(stripANSI(detail))
		if title == "" || !strings.Contains(compact, compactProgressDetailText(fullPath)) || !strings.Contains(compact, compactProgressDetailText(fullError)) {
			t.Fatalf("width %d portal lost full source detail, title=%q:\n%s", width, title, stripANSI(detail))
		}
		for row, line := range strings.Split(detail, "\n") {
			if got := lipgloss.Width(line); got > m.portal.contentWidth() {
				t.Fatalf("width %d detail row %d rendered %d cells", width, row+1, got)
			}
		}
	}
}

func denseRemovalSummaryModel() Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = summaryView
	m.width, m.windowHeight, m.height = 80, 24, 18
	m.portal = m.portal.SetSize(80, 24)
	m.remove.run.result.FailureCount = 30
	for i := 0; i < 30; i++ {
		m.remove.run.result.Outcomes = append(m.remove.run.result.Outcomes, worktree.WorktreeOutcome{
			Path:    "/repo/worktrees/failed-" + itoa(i),
			Success: false,
			Error:   errors.New("removal failed " + itoa(i)),
		})
	}
	return m
}

func TestViewSummary_DenseFailuresStayWithinTerminalAndKeepFooter(t *testing.T) {
	m := denseRemovalSummaryModel()
	view := stripANSI(m.viewSummary())
	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")

	if len(lines) > m.windowHeight {
		t.Fatalf("summary has %d rows, terminal has %d", len(lines), m.windowHeight)
	}
	if !strings.Contains(lines[len(lines)-1], "q quit") {
		t.Fatalf("visible footer missing quit action: %q", lines[len(lines)-1])
	}
}

func TestViewSummary_DenseFailuresExposeOmittedResultsInDetails(t *testing.T) {
	m := denseRemovalSummaryModel()
	view := stripANSI(m.viewSummary())
	if !strings.Contains(view, "? details") {
		t.Fatalf("bounded summary does not advertise omitted details:\n%s", view)
	}

	title, detail := m.detailContent()
	if title == "" || !strings.Contains(stripANSI(detail), "failed-29") {
		t.Fatalf("detail portal lost final omitted failure: title=%q\n%s", title, stripANSI(detail))
	}
	for row, line := range strings.Split(detail, "\n") {
		if width := ansi.StringWidth(line); width > m.portal.contentWidth() {
			t.Fatalf("detail row %d width=%d exceeds portal width=%d", row+1, width, m.portal.contentWidth())
		}
	}
}

func TestUpdateSummary_MenuLaunch_KeysReturnToMenu(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyPressMsg
	}{
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}},
		{"esc", tea.KeyPressMsg{Code: tea.KeyEsc}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
			m.view = summaryView

			updated, cmd := m.updateSummary(tc.msg)

			if updated.(Model).view != menuView {
				t.Errorf("view = %d, want menuView", updated.(Model).view)
			}
			if cmd != nil {
				t.Error("returning to menu should not emit a command")
			}
		})
	}
}

func TestUpdateSummary_MenuLaunch_QuitKeyMatchesFooter(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = summaryView

	updated, cmd := m.updateSummary(keyMsg("q"))
	if updated.(Model).view != summaryView {
		t.Fatalf("quit key changed view to %d before exit", updated.(Model).view)
	}
	if cmd == nil {
		t.Fatal("footer advertises q quit, but q emitted no quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q emitted %T, want tea.QuitMsg", cmd())
	}
}

func TestUpdateSummary_DirectLaunch_KeysQuit(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyPressMsg
	}{
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}},
		{"quit key", keyMsg("q")},
		{"esc", tea.KeyPressMsg{Code: tea.KeyEsc}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(nil, nil, "/repo")
			m.view = summaryView

			_, cmd := m.updateSummary(tc.msg)

			if cmd == nil {
				t.Fatal("expected a quit command")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("expected tea.QuitMsg, got %T", cmd())
			}
		})
	}
}

func TestUpdateSummary_OtherKeysIgnored(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.view = summaryView

	updated, cmd := m.updateSummary(keyMsg("x"))

	if updated.(Model).view != summaryView || cmd != nil {
		t.Error("unhandled keys should leave the model untouched")
	}
}
