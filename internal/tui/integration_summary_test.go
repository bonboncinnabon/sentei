package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

func demoIntegrationFailureEvents() []progress.Event {
	var events []progress.Event
	for _, worktree := range []string{"/repo/worktrees/alpha", "/repo/worktrees/beta", "/repo/worktrees/gamma"} {
		events = append(events,
			progress.Event{Phase: worktree, PhaseLabel: worktree, Step: "setup", StepLabel: "Setup cocoindex-code", Status: progress.StepPending, Of: 1},
			progress.Event{Phase: worktree, PhaseLabel: worktree, Step: "gitignore", StepLabel: "Update .gitignore for cocoindex-code", Status: progress.StepPending, Of: 1},
			progress.Event{Phase: worktree, PhaseLabel: worktree, Close: true},
			progress.Event{Phase: worktree, Step: "setup", Status: progress.StepFailed, Error: errors.New("ccc init && ccc index: deterministic index failure\nexit status 17")},
			progress.Event{Phase: worktree, Step: "gitignore", Status: progress.StepSkipped, Message: "blocked by Setup cocoindex-code"},
		)
	}
	return events
}

func TestViewIntegrationSummary_DemoFailuresFitEveryTerminal(t *testing.T) {
	for _, width := range []int{20, 40, 80, 120} {
		for height := 1; height <= 40; height++ {
			t.Run(fmt.Sprintf("%dx%d", width, height), func(t *testing.T) {
				m := makeIntegrationModel()
				m.view = integrationSummaryView
				m.width, m.windowHeight, m.height = width, height, max(height-viewChromeRows, 5)
				m.integ.events = demoIntegrationFailureEvents()

				view := m.viewIntegrationSummary()
				lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
				if got := lipgloss.Height(view); got > height {
					t.Fatalf("summary has %d rows, terminal has %d:\n%s", got, height, stripAnsi(view))
				}
				for row, line := range lines {
					if got := ansi.StringWidth(line); got > width {
						t.Fatalf("row %d width=%d exceeds %d: %q", row+1, got, width, stripAnsi(line))
					}
				}
			})
		}
	}
}

func TestViewIntegrationSummary_TopLevelErrorRemainsActionableWhenCompact(t *testing.T) {
	longError := "manifest is malformed: " + strings.Repeat("dependency-resolution-context-", 8)
	for _, height := range []int{3, 4} {
		m := makeIntegrationModel()
		m.view = integrationSummaryView
		m.width, m.windowHeight = 80, height
		m.portal = m.portal.SetSize(80, 24)
		m.integ.prepareErr = errors.New(longError)

		view := stripAnsi(m.viewIntegrationSummary())
		if !strings.Contains(view, "manifest is malformed") && !strings.Contains(view, "details") {
			t.Fatalf("height %d hid the top-level error without a details affordance:\n%s", height, view)
		}
		_, detail := m.integrationSummaryDetailContent()
		if !strings.Contains(stripAnsi(detail), "manifest is malformed") {
			t.Fatalf("height %d detail portal lost top-level error: %q", height, stripAnsi(detail))
		}
		for row, line := range strings.Split(detail, "\n") {
			if got := ansi.StringWidth(line); got > m.portal.contentWidth() {
				t.Fatalf("height %d detail row %d width=%d exceeds portal width=%d: %q", height, row+1, got, m.portal.contentWidth(), stripAnsi(line))
			}
		}
	}
}

func TestIntegrationSummaryDetail_WrapsLongStepErrorsToPortalWidth(t *testing.T) {
	longError := strings.Repeat("index dependency resolution failed ", 12)
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.width, m.windowHeight = 40, 24
	m.portal = m.portal.SetSize(40, 24)
	m.integ.events = []progress.Event{
		{Phase: "worktree", PhaseLabel: "worktree", Step: "index", StepLabel: "Index", Status: progress.StepPending, Of: 1},
		{Phase: "worktree", PhaseLabel: "worktree", Close: true},
		{Phase: "worktree", Step: "index", Status: progress.StepFailed, Error: errors.New(longError)},
	}

	_, detail := m.integrationSummaryDetailContent()
	if !strings.Contains(compactProgressDetailText(stripAnsi(detail)), compactProgressDetailText(longError)) {
		t.Fatalf("wrapped detail lost step error:\n%s", stripAnsi(detail))
	}
	for row, line := range strings.Split(detail, "\n") {
		if got := ansi.StringWidth(line); got > m.portal.contentWidth() {
			t.Fatalf("detail row %d width=%d exceeds portal width=%d: %q", row+1, got, m.portal.contentWidth(), stripAnsi(line))
		}
	}
}

func TestViewIntegrationSummary_DemoFailuresRemainCompleteInDetails(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.width, m.windowHeight = 80, 24
	m.integ.events = demoIntegrationFailureEvents()

	view := stripAnsi(m.viewIntegrationSummary())
	if !strings.Contains(view, "3 failed") || !strings.Contains(view, "details") {
		t.Fatalf("bounded summary lost verdict or details hint:\n%s", view)
	}
	_, detail := m.integrationSummaryDetailContent()
	plain := stripAnsi(detail)
	for _, want := range []string{"alpha", "beta", "gamma", "deterministic index failure", "exit status 17", "blocked by Setup cocoindex-code"} {
		if !strings.Contains(compactProgressDetailText(plain), compactProgressDetailText(want)) {
			t.Errorf("detail portal missing %q:\n%s", want, plain)
		}
	}
}

func TestIntegrationProgressToSummary_DropsOutgoingBarContent(t *testing.T) {
	t.Setenv("SENTEI_MOTION", "off")
	m := makeIntegrationModel()
	m.width, m.windowHeight, m.height = 80, 24, 18
	m.view = integrationProgressView
	m.integ.lifecycle = integrationExecuting
	m.integ.events = demoIntegrationFailureEvents()
	progressView := stripAnsi(m.viewIntegrationProgress())
	if !strings.Contains(progressView, "%") {
		t.Fatal("progress precondition missing bar percentage")
	}

	m.view = integrationSummaryView
	m.integ.lifecycle = integrationSettling
	summary := stripAnsi(m.viewIntegrationSummary())
	if strings.Contains(summary, "%") || strings.Contains(summary, "█") || strings.Contains(summary, "░") {
		t.Fatalf("summary content retained outgoing progress bar text:\n%s", summary)
	}
}

func doneEventsForWorktrees(n int) []progress.Event {
	var events []progress.Event
	for i := 0; i < n; i++ {
		events = append(events, progress.Event{
			Phase:  fmt.Sprintf("/repo/feature-%d", i),
			Step:   "Setup code-review-graph",
			Status: progress.StepDone,
		})
	}
	return events
}

func TestUpdateIntegrationProgress_Finalized_TransitionsToSummary(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = integrationListView
	genBefore := m.worktreeGeneration

	updated, cmd := m.updateIntegrationProgress(integrationFinalizedMsg{err: nil})
	m = updated.(Model)

	m = settleNow(t, m)
	if m.view != integrationSummaryView {
		t.Errorf("expected integrationSummaryView, got %d", m.view)
	}
	if m.integ.saveErr != nil {
		t.Errorf("expected nil saveErr, got %v", m.integ.saveErr)
	}
	if m.worktreeGeneration != genBefore+1 {
		t.Error("expected eager worktree reload generation bump on successful apply")
	}
	if cmd == nil {
		t.Error("expected reload Cmd alongside summary transition")
	}
}

func TestUpdateIntegrationProgress_Finalized_SaveError_TransitionsToSummary(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = integrationListView
	genBefore := m.worktreeGeneration

	saveErr := errors.New("save failed")
	updated, _ := m.updateIntegrationProgress(integrationFinalizedMsg{err: saveErr})
	m = updated.(Model)

	m = settleNow(t, m)
	if m.view != integrationSummaryView {
		t.Errorf("expected integrationSummaryView on save failure, got %d", m.view)
	}
	if !errors.Is(m.integ.saveErr, saveErr) {
		t.Errorf("expected saveErr stored, got %v", m.integ.saveErr)
	}
	if m.worktreeGeneration != genBefore {
		t.Error("worktreeGeneration must not advance on a failed save")
	}
	if m.integ.staged["cocoindex-code"] {
		t.Error("a failed save must not mutate staged state")
	}
}

func TestUpdateIntegrationSummary_Enter_ReturnsToListAndReloads(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.integ.lifecycle = integrationSettling

	updated, cmd := m.updateIntegrationSummary(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != integrationListView {
		t.Errorf("expected integrationListView, got %d", model.view)
	}
	if cmd == nil {
		t.Error("expected loadIntegrationState Cmd so staged markers reconcile from disk")
	}
	if model.integ.lifecycle != integrationIdle {
		t.Fatalf("lifecycle=%v, want idle after dismissing summary", model.integ.lifecycle)
	}
}

func TestViewIntegrationSummary_AllSucceeded(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.integ.events = []progress.Event{
		{Phase: "/repo/feature-a", Step: "Setup code-review-graph", Status: progress.StepRunning},
		{Phase: "/repo/feature-a", Step: "Setup code-review-graph", Status: progress.StepDone},
		{Phase: "/repo/feature-b", Step: "Setup code-review-graph", Status: progress.StepDone},
	}

	view := stripAnsi(m.viewIntegrationSummary())
	for _, want := range []string{"feature-a", "feature-b", indicatorDone, "2 steps applied", "enter integrations"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected summary to contain %q, view:\n%s", want, view)
		}
	}
	if strings.Contains(view, "failed") {
		t.Errorf("fully successful apply must not mention failures, view:\n%s", view)
	}
}

func TestViewIntegrationSummary_EmptyPlanShowsNoWorkVerdict(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.integ.events = nil

	view := stripAnsi(m.viewIntegrationSummary())
	if !strings.Contains(view, "No integration work was needed") {
		t.Fatalf("explicit no-work verdict missing:\n%s", view)
	}
	if strings.Contains(view, "0 steps applied") {
		t.Fatalf("empty plan must not render a green zero-step verdict:\n%s", view)
	}
}

func TestViewIntegrationSummary_PartialFailure(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.integ.events = []progress.Event{
		{Phase: "/repo/feature-a", Step: "Setup code-review-graph", Status: progress.StepDone},
		{Phase: "/repo/feature-b", Step: "Install dependency pipx", Status: progress.StepFailed, Error: errors.New("brew install pipx: exit 1")},
	}

	view := m.viewIntegrationSummary()
	for _, want := range []string{"1 step applied", "1 failed", indicatorFailed, "brew install pipx: exit 1"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected summary to contain %q, view:\n%s", want, view)
		}
	}
}

func TestViewIntegrationSummary_SaveError(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.integ.events = []progress.Event{
		{Phase: "/repo/feature-a", Step: "Setup code-review-graph", Status: progress.StepDone},
	}
	m.integ.saveErr = errors.New("disk full")

	view := m.viewIntegrationSummary()
	if !strings.Contains(view, "not saved") {
		t.Errorf("expected prominent not-saved warning, view:\n%s", view)
	}
	if !strings.Contains(view, "disk full") {
		t.Errorf("expected save error detail, view:\n%s", view)
	}
}

func TestViewIntegrationSummary_PreparationErrorUsesErrorVerdict(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.integ.prepareErr = errors.New("no target worktree")

	view := stripAnsi(m.viewIntegrationSummary())
	if !strings.Contains(view, titleApplyErrors) {
		t.Fatalf("preparation error must use error title, view:\n%s", view)
	}
	if !strings.Contains(view, "Integration preparation failed: no target worktree") {
		t.Fatalf("preparation error headline missing, view:\n%s", view)
	}
	if strings.Contains(view, "0 steps applied") {
		t.Fatalf("preparation error must not render a green zero-success verdict, view:\n%s", view)
	}
}

func TestViewIntegrationSummary_OnlyFailuresHasOneErrorVerdictAndNoZeroSuccess(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.integ.events = []progress.Event{{
		Phase: "/repo/main", Step: "Setup tool", Status: progress.StepFailed, Error: errors.New("boom"),
	}}

	view := stripAnsi(m.viewIntegrationSummary())
	if strings.Count(view, "1 failed") != 1 {
		t.Fatalf("expected exactly one failure verdict:\n%s", view)
	}
	if strings.Contains(view, "0 steps applied") || strings.Contains(view, "0 step applied") {
		t.Fatalf("error summary must not render a green zero-success verdict:\n%s", view)
	}
}

func TestViewIntegrationSummary_OverflowPeeksAndOffersDetails(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationSummaryView
	m.width, m.height = 80, 40
	m.integ.events = doneEventsForWorktrees(5)

	view := stripAnsi(m.viewIntegrationSummary())

	for _, want := range []string{"feature-0", "feature-1", "feature-2"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected peek to show %q, view:\n%s", want, view)
		}
	}
	for _, hidden := range []string{"feature-3", "feature-4"} {
		if strings.Contains(view, hidden) {
			t.Errorf("expected %q to be hidden behind the portal, view:\n%s", hidden, view)
		}
	}
	if !strings.Contains(view, "and 2 more") {
		t.Errorf("expected an 'and N more' overflow line, view:\n%s", view)
	}
	if !strings.Contains(view, "details") {
		t.Errorf("expected a `?` details hint when outcomes overflow, view:\n%s", view)
	}
}

func TestIntegrationSummaryDetailContent(t *testing.T) {
	tests := []struct {
		name        string
		worktrees   int
		wantContent bool
	}{
		{"within peek has no portal", inlineSummaryPreview, false},
		{"overflow exposes full breakdown", inlineSummaryPreview + 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeIntegrationModel()
			m.view = integrationSummaryView
			m.width, m.height = 80, 40
			m.integ.events = doneEventsForWorktrees(tt.worktrees)

			title, content := m.detailContent()
			if tt.wantContent {
				if content == "" {
					t.Fatal("expected portal content when outcomes overflow the peek")
				}
				if title != "Apply details" {
					t.Errorf("title = %q, want %q", title, "Apply details")
				}
				for i := 0; i < tt.worktrees; i++ {
					if !strings.Contains(stripAnsi(content), fmt.Sprintf("feature-%d", i)) {
						t.Errorf("portal must list every worktree; missing feature-%d:\n%s", i, content)
					}
				}
			} else if content != "" {
				t.Errorf("expected no portal content within the peek, got:\n%s", content)
			}
		})
	}
}
