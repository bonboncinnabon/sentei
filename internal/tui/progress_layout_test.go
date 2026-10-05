package tui

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

func TestProgressLayout_WithSubtitle(t *testing.T) {
	l := ProgressLayout{
		Title:    "Creating worktree",
		Subtitle: "feature/foo → from main",
		Width:    80,
		Height:   30,
		Phases: []progress.PhaseState{
			{Name: "Setup", Total: 2, Done: 1, Steps: []progress.StepState{
				{Name: "Create worktree", Status: progress.StepDone},
				{Name: "Merge base branch", Status: progress.StepRunning},
			}},
			{Name: "Dependencies"},
			{Name: "Integrations"},
		},
	}
	view := stripANSI(l.View())

	for _, want := range []string{"sentei ─ Creating worktree", "feature/foo → from main", "┄", "Setup", "2 phases waiting"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected layout to contain %q, view:\n%s", want, view)
		}
	}
}

func TestProgressLayout_WithoutSubtitle(t *testing.T) {
	l := ProgressLayout{Title: "Removing worktrees", Width: 80, Height: 30}
	view := stripANSI(l.View())

	lines := strings.Split(view, "\n")
	if !strings.Contains(lines[0], "sentei ─ Removing worktrees") {
		t.Errorf("expected title first, got %q", lines[0])
	}
	if strings.TrimSpace(lines[1]) != "" {
		t.Errorf("expected blank line directly after title when no subtitle, got %q", lines[1])
	}
}

func TestProgressLayout_CompletedPhaseCollapses(t *testing.T) {
	l := ProgressLayout{
		Title: "T", Width: 80, Height: 30,
		Phases: []progress.PhaseState{
			{Name: "Setup", Total: 2, Done: 2, Closed: true, Steps: []progress.StepState{
				{Name: "Create worktree", Status: progress.StepDone},
				{Name: "Merge base branch", Status: progress.StepDone},
			}},
		},
	}
	view := stripANSI(l.View())

	if !strings.Contains(view, "✦ Setup  2/2  100%") {
		t.Errorf("expected collapsed completed phase, view:\n%s", view)
	}
	if strings.Contains(view, "Create worktree") {
		t.Errorf("completed phase must not list steps, view:\n%s", view)
	}
}

func TestProgressLayout_CompletedPhaseWithFailuresKeepsSteps(t *testing.T) {
	l := ProgressLayout{
		Title: "T", Width: 80, Height: 30,
		Phases: []progress.PhaseState{
			{Name: "Setup", Total: 2, Done: 2, Failed: 1, Steps: []progress.StepState{
				{Name: "Create worktree", Status: progress.StepDone},
				{Name: "Install hooks", Status: progress.StepFailed},
			}},
		},
	}
	view := stripANSI(l.View())

	if !strings.Contains(view, "Install hooks") {
		t.Errorf("failed steps must stay visible after phase completion, view:\n%s", view)
	}
	if !strings.Contains(view, indicatorFailed) {
		t.Errorf("expected failed indicator, view:\n%s", view)
	}
}

func TestProgressLayout_ActivePhaseShowsSteps(t *testing.T) {
	l := ProgressLayout{
		Title: "T", Width: 80, Height: 30,
		Phases: []progress.PhaseState{
			{Name: "Removing worktrees", Total: 30, Done: 12, Steps: []progress.StepState{
				{Name: "done-step", Status: progress.StepDone},
				{Name: "active-step", Status: progress.StepRunning},
				{Name: "pending-step", Status: progress.StepPending},
			}},
		},
	}
	view := stripANSI(l.View())

	if !strings.Contains(view, "✻ Removing worktrees  12/30  40%") {
		t.Errorf("expected active phase header with indicator left and count/pct, view:\n%s", view)
	}
	for _, want := range []string{"    ✦ done-step", "    ✻ active-step", "    · pending-step"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected 4-space indented step %q, view:\n%s", want, view)
		}
	}
}

func TestProgressLayout_ZeroTotalPhaseIsPending(t *testing.T) {
	l := ProgressLayout{
		Title: "T", Width: 80, Height: 30,
		Phases: []progress.PhaseState{
			{Name: "Integrations", Total: 0, Done: 0, Steps: []progress.StepState{
				{Name: "queued work", Status: progress.StepPending},
			}},
		},
	}
	view := stripANSI(l.View())

	if strings.Contains(view, "100%") {
		t.Errorf("a 0/0 phase must never render complete, view:\n%s", view)
	}
	if !strings.Contains(view, "pending") {
		t.Errorf("expected pending phase, view:\n%s", view)
	}
}

func TestProgressLayout_OverallBarAggregatesPhases(t *testing.T) {
	l := ProgressLayout{
		Title: "T", Width: 80, Height: 30,
		Phases: []progress.PhaseState{
			{Name: "A", Total: 12, Done: 8},
			{Name: "B", Total: 8, Done: 2},
		},
	}
	view := stripANSI(l.View())

	barWidth := 80 - 2 - progressBarPercentReserve
	want := strings.Repeat("█", barWidth/2) + strings.Repeat("░", barWidth-barWidth/2) + " 50%"
	if !strings.Contains(view, want) {
		t.Errorf("expected aggregated 50%% bar, view:\n%s", view)
	}
}

func TestProgressLayout_DoneExceedsTotal_NoPanic(t *testing.T) {
	l := ProgressLayout{
		Title: "T", Width: 80, Height: 30,
		Phases: []progress.PhaseState{
			{Name: "A", Total: 1, Done: 3},
		},
	}
	view := stripANSI(l.View()) // must not panic

	if !strings.Contains(view, "100%") {
		t.Errorf("expected clamped 100%%, view:\n%s", view)
	}
	if strings.Contains(view, "300%") {
		t.Errorf("percentage must clamp, view:\n%s", view)
	}
}

func TestProgressLayout_WindowsStepsOnShortTerminal(t *testing.T) {
	steps := make([]progress.StepState, 30)
	for i := range steps {
		steps[i] = progress.StepState{Name: stepName(i), Status: progress.StepPending}
	}
	steps[0].Status = progress.StepRunning
	l := ProgressLayout{
		Title: "T", Width: 80, Height: 15,
		Phases: []progress.PhaseState{{Name: "Removing", Total: 30, Done: 0, Steps: steps}},
	}
	view := stripANSI(l.View())

	if !strings.Contains(view, "showing") {
		t.Errorf("expected stat line when windowed at height 15, view:\n%s", view)
	}
}

func TestProgressLayout_HintsRendered(t *testing.T) {
	l := ProgressLayout{
		Title: "T", Width: 80, Height: 30,
		Hints: progressFooter,
	}
	view := stripANSI(l.View())

	if !strings.Contains(view, "  q quit") {
		t.Errorf("expected key hints, view:\n%s", view)
	}
}
