package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
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

// A run that ended before its prune must not claim one: a removal that could
// not even be prepared goes straight to the summary.
func TestViewSummary_ClaimsNoPruneWhenNoneRan(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/repo/a", Branch: "refs/heads/feature/a"},
		{Path: "/repo/x/../a", Branch: "refs/heads/feature/a"},
	}, nil, "/repo")
	m.remove.selected = map[string]bool{"/repo/a": true, "/repo/x/../a": true}
	m.view = confirmView

	updated, _ := m.Update(keyRune('y'))
	m = updated.(Model)
	if m.view != summaryView || m.remove.run.pruned {
		t.Fatalf("view=%d pruned=%v, want a summary for a run that never pruned", m.view, m.remove.run.pruned)
	}
	if summary := stripAnsi(m.viewSummary()); strings.Contains(summary, "Pruned") {
		t.Errorf("summary claims a prune that never ran:\n%s", summary)
	}
}

func TestViewSummary_FailureRowsAndContinuationLinesShareOneIndent(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.view = summaryView
	m.width, m.windowHeight, m.height = 100, 40, 34
	m.remove.run.pruned = true
	m.remove.run.pruneErr = errors.New("git worktree prune: error: first\nerror: second")
	m.remove.run.result.SuccessCount = 1
	m.remove.run.result.FailureCount = 2
	m.remove.run.result.Err = errors.Join(errors.New("progress: first"), errors.New("progress: second"))
	m.remove.run.result.Outcomes = []worktree.WorktreeOutcome{
		{Path: "/repo/a", Error: errors.New("git worktree remove --force /repo/a: error: failed to delete 'x'\r\nerror: failed to delete 'y'")},
		{Path: "/repo/done", Success: true},
		{Path: "/repo/b", Error: errors.New("permission denied")},
	}
	m.remove.run.result.Phases = []progress.Phase{
		{ID: worktree.RemovalPhaseID, Name: worktree.RemovalPhaseName, Steps: []progress.StepResult{
			{Name: "a", Status: progress.StepFailed, Error: errors.New("listed above as an outcome")},
		}},
		{ID: "other", Name: "Other", Steps: []progress.StepResult{
			{Name: "first", Status: progress.StepFailed, Error: errors.New("fatal: one\nhint: two")},
			{Name: "second", Status: progress.StepFailed, Error: errors.New("fatal: three")},
		}},
	}

	view := stripANSI(m.viewSummary())
	want := strings.Join([]string{
		"  1 removed, 2 failed",
		"",
		"  Removal failures:",
		"    ✗ /repo/a: git worktree remove --force /repo/a: error: failed to delete 'x'",
		"      error: failed to delete 'y'",
		"    ✗ /repo/b: permission denied",
		"",
		"  ✗ progress: first",
		"    progress: second",
		"",
		"  Execution failures:",
		"    ✗ Other / first: fatal: one",
		"      hint: two",
		"    ✗ Other / second: fatal: three",
		"",
		"  Warning: failed to prune worktree metadata: git worktree prune: error: first",
		"  error: second",
		"  Run `git worktree prune` in the repository to retry.",
		"",
	}, "\n")
	if !strings.Contains(view, want) {
		t.Fatalf("failure block is not laid out as\n%s\n\ngot:\n%s", want, view)
	}
}

func TestRemovalSummaryDetail_WrappedFailureRowsContinueUnderTheirText(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.view = summaryView
	m.width, m.windowHeight, m.height = 60, 40, 34
	m.portal = m.portal.SetSize(60, 40)
	m.remove.run.result.FailureCount = 1
	m.remove.run.result.Outcomes = []worktree.WorktreeOutcome{{
		Path:  "/repo/" + strings.Repeat("long-worktree-name-", 6),
		Error: errors.New("error: failed to delete\nhint: " + strings.Repeat("fix the permissions ", 6)),
	}}

	_, detail := m.detailContent()
	lines := strings.Split(stripANSI(detail), "\n")
	start := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, "    ✗ ") })
	if start < 0 {
		t.Fatalf("detail has no failure row:\n%s", stripANSI(detail))
	}
	continued := 0
	for _, line := range lines[start+1:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		continued++
		if !strings.HasPrefix(line, "      ") {
			t.Fatalf("continuation line starts left of the row's text: %q\n%s", line, stripANSI(detail))
		}
	}
	if continued < 4 {
		t.Fatalf("want the path, the error and the hint to wrap, got %d continuation lines:\n%s", continued, stripANSI(detail))
	}
}
