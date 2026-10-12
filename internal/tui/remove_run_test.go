package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func keyRune(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// pollutedRun returns run state as a completed previous deletion left it:
// two outcomes, done statuses, prune and cleanup results all set.
func pollutedRun() removalRun {
	return removalRun{
		result: worktree.DeletionResult{
			SuccessCount: 2,
			Outcomes: []worktree.WorktreeOutcome{
				{Path: "/work/a", Success: true},
				{Path: "/work/b", Success: true},
			},
		},
		pruned:   true,
		pruneErr: errors.New("prune failed last time"),
	}
}

func TestConfirmYes_SecondRunStartsFresh(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/a"},
		{Path: "/work/b", Branch: "refs/heads/b"},
		{Path: "/work/c", Branch: "refs/heads/c"},
	}, nil, "/repo")
	m.remove.run = pollutedRun()
	m.remove.selected = map[string]bool{"/work/c": true}
	m.view = confirmView

	updated, _ := m.updateConfirm(keyRune('y'))
	model := updated.(Model)

	run := model.remove.run
	if got := len(run.result.Outcomes); got != 0 {
		t.Errorf("expected 0 outcomes in fresh run, got %d", got)
	}
	if run.result.SuccessCount != 0 || run.result.FailureCount != 0 {
		t.Errorf("expected zero counts in fresh run, got %+v", run.result)
	}
	if got := len(run.targets); got != 1 || run.targets[0].Worktree.Path != "/work/c" {
		t.Errorf("expected the fresh run to target only /work/c, got %+v", run.targets)
	}
	if run.pruned || run.pruneErr != nil {
		t.Errorf("expected the prune outcome cleared in a fresh run, got pruned=%v pruneErr=%v", run.pruned, run.pruneErr)
	}

	view := model.viewProgress()
	if strings.Contains(view, "200%") {
		t.Error("progress view shows 200% from stale outcomes")
	}
	if !strings.Contains(view, "0%") {
		t.Errorf("expected fresh run to start at 0%%, view:\n%s", view)
	}
	phases := model.buildRemovalPhases()
	if len(phases) != 1 || phases[0].Name != worktree.RemovalPhaseName || phases[0].Done != 0 {
		t.Errorf("expected only a pending removal phase on a fresh run, got %+v", phases)
	}
}

func TestViewProgress_SecondRunCompletesAtHundredPercent(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/c", Branch: "refs/heads/c"},
	}, nil, "/repo")
	m.remove.run = removalRun{}
	m.view = progressView

	updated, _ := m.updateProgress(removalEventMsg{event: progress.Event{Phase: worktree.RemovalPhaseName, Step: "/work/c", Status: progress.StepDone}})
	model := updated.(Model)

	view := model.viewProgress()
	if !strings.Contains(view, "100%") {
		t.Errorf("expected 100%% after the run's single deletion, view:\n%s", view)
	}
}

func TestUpdateProgress_TerminalDeletionResultIsNotDoubleCountedByTrailingEvents(t *testing.T) {
	wt := git.Worktree{Path: "/work/a", Branch: "refs/heads/a"}
	m := NewModel([]git.Worktree{wt}, nil, "/repo")
	m.remove.run = removalRun{}
	m.remove.run.targets = []worktree.RemovalTarget{{Worktree: wt, StepID: "remove-a"}}
	m.remove.run.progressCh = make(chan progress.Event)
	m.view = progressView

	terminal := worktree.DeletionResult{
		SuccessCount: 1,
		Outcomes:     []worktree.WorktreeOutcome{{Path: wt.Path, Success: true}},
	}
	updated, _ := m.updateProgress(deletionsCompleteMsg{Result: terminal})
	m = updated.(Model)
	updated, _ = m.updateProgress(removalEventMsg{event: progress.Event{
		Phase: worktree.RemovalPhaseID, Step: "remove-a", Status: progress.StepDone,
	}})
	m = updated.(Model)

	if m.remove.run.result.SuccessCount != 1 || len(m.remove.run.result.Outcomes) != 1 {
		t.Fatalf("trailing terminal event double-counted result: %+v", m.remove.run.result)
	}
}

func TestUpdateProgress_EventsComplete_ClearsSelection(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/a"},
	}, nil, "/repo")
	m.remove.selected = map[string]bool{"/work/a": true}
	m.view = progressView

	updated, _ := m.updateProgress(removalEventsCompleteMsg{})
	model := updated.(Model)

	if got := len(model.remove.selected); got != 0 {
		t.Errorf("expected selection cleared after completed run, got %d selected", got)
	}
}

func TestMenuEntry_RemoveWorktrees_ClearsSelection(t *testing.T) {
	m := NewModel([]git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/a"},
	}, nil, "/repo")
	m.menuItems = []menuItem{{label: "Remove worktrees", enabled: true}}
	m.menuCursor = 0
	m.remove.selected = map[string]bool{"/work/a": true}
	m.view = menuView

	updated, _ := m.updateMenu(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != listView {
		t.Fatalf("expected listView, got %d", model.view)
	}
	if got := len(model.remove.selected); got != 0 {
		t.Errorf("expected selection cleared on menu entry, got %d selected", got)
	}
}

func TestViewSummary_FailedOutcome_ReadsFromRun(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.remove.run = removalRun{}
	m.remove.run.result = worktree.DeletionResult{
		SuccessCount: 1,
		FailureCount: 1,
		Outcomes: []worktree.WorktreeOutcome{
			{Path: "/work/a", Success: true},
			{Path: "/work/b", Success: false, Error: errors.New("boom")},
		},
	}
	m.view = summaryView

	view := m.viewSummary()
	if !strings.Contains(view, "1 removed") || !strings.Contains(view, "1 failed") {
		t.Errorf("expected counts from run result, view:\n%s", view)
	}
	if !strings.Contains(view, "boom") {
		t.Errorf("expected failure detail from run result, view:\n%s", view)
	}
}
