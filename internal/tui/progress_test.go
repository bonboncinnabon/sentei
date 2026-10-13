package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func TestViewSummary_ExecutionFailureOverridesSuccessfulDeletionCopy(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.remove.run.result = worktree.DeletionResult{
		SuccessCount: 1,
		Phases:       []progress.Phase{{Name: "Unlock", Steps: []progress.StepResult{{Name: "feature/a", Status: progress.StepFailed, Error: errors.New("unlock denied")}}}},
	}
	m.view = summaryView

	view := stripANSI(m.viewSummary())
	if strings.Contains(view, "removed successfully") {
		t.Fatalf("summary advertised success despite failed execution:\n%s", view)
	}
	if !strings.Contains(view, "1 removed") || !strings.Contains(view, "unlock denied") {
		t.Fatalf("summary lost deletion count or execution error:\n%s", view)
	}
}

func TestViewSummary_DeliveryErrorIsVisible(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.remove.run.result = worktree.DeletionResult{SuccessCount: 1, Err: errors.New("progress delivery failed")}
	m.view = summaryView

	view := stripANSI(m.viewSummary())
	if strings.Contains(view, "removed successfully") || !strings.Contains(view, "progress delivery failed") {
		t.Fatalf("delivery failure not visible:\n%s", view)
	}
}

type stubRunner struct {
	responses map[string]stubResponse
}

type stubResponse struct {
	output string
	err    error
}

func (s *stubRunner) Run(dir string, args ...string) (string, error) {
	key := dir
	for _, a := range args {
		key += " " + a
	}
	resp, ok := s.responses[key]
	if !ok {
		return "", fmt.Errorf("unexpected command: %s", key)
	}
	return resp.output, resp.err
}

func TestUpdateProgress_DeletionsComplete_TriggersPrune(t *testing.T) {
	runner := &stubRunner{
		responses: map[string]stubResponse{
			"/repo worktree prune": {output: ""},
		},
	}

	m := NewModel([]git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/a"},
	}, runner, "/repo")
	m.view = progressView

	updated, cmd := m.updateProgress(deletionsCompleteMsg{})

	if cmd == nil {
		t.Fatal("expected a Cmd from allDeletionsCompleteMsg, got nil")
	}

	model := updated.(Model)
	if model.view == summaryView {
		t.Error("should not transition to summaryView yet — prune should run first")
	}

	// The cmd batches the spring sync with the prune; find the prune result.
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", cmd())
	}
	found := false
	for _, c := range batch {
		if c == nil {
			continue
		}
		if pruneMsg, ok := c().(pruneCompleteMsg); ok {
			found = true
			if pruneMsg.Err != nil {
				t.Errorf("expected prune success, got error: %v", pruneMsg.Err)
			}
		}
	}
	if !found {
		t.Fatal("expected pruneCompleteMsg in the batch")
	}
}

func TestUpdateProgress_PruneComplete_EndsTheRun(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView

	updated, _ := m.updateProgress(pruneCompleteMsg{Err: nil})
	model := updated.(Model)

	if !model.remove.run.pruned || model.remove.run.pruneErr != nil {
		t.Errorf("run after a clean prune: pruned=%v pruneErr=%v, want true and nil", model.remove.run.pruned, model.remove.run.pruneErr)
	}
	if !model.removalLayout().Completed {
		t.Error("the prune is the run's last work: the layout must read as completed")
	}
	if model.view == summaryView {
		t.Error("the summary must wait for the event stream to drain")
	}
}

func TestUpdateProgress_EventsComplete_TransitionsToSummary(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView

	updated, _ := m.updateProgress(removalEventsCompleteMsg{})
	model := settleNow(t, updated.(Model))

	if model.view != summaryView {
		t.Errorf("expected summaryView, got %d", model.view)
	}
}

func TestUpdateProgress_PruneFailed_StaysOutOfTheResult(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView

	pruneError := fmt.Errorf("permission denied")
	updated, _ := m.updateProgress(pruneCompleteMsg{Err: pruneError})
	model := updated.(Model)

	if !errors.Is(model.remove.run.pruneErr, pruneError) {
		t.Errorf("pruneErr = %v, want the prune error", model.remove.run.pruneErr)
	}
	if model.remove.run.result.HasFailures() {
		t.Errorf("a failed prune marked the removal result as failed: %+v", model.remove.run.result)
	}
}

func TestViewSummary_PruneSuccess(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = summaryView
	m.remove.run.result = worktree.DeletionResult{
		SuccessCount: 2,
		FailureCount: 0,
		Outcomes: []worktree.WorktreeOutcome{
			{Path: "/a", Success: true},
			{Path: "/b", Success: true},
		},
	}
	m.remove.run.pruned = true

	output := stripAnsi(m.viewSummary())

	if !strings.Contains(output, "Pruned orphaned worktree metadata") {
		t.Error("should show prune success message")
	}
	if strings.Contains(output, "Warning") {
		t.Error("should not show warning on prune success")
	}
}

func TestViewSummary_PruneFailure(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = summaryView
	m.remove.run.result = worktree.DeletionResult{
		SuccessCount: 1,
		FailureCount: 0,
		Outcomes: []worktree.WorktreeOutcome{
			{Path: "/a", Success: true},
		},
	}
	m.remove.run.pruned = true
	m.remove.run.pruneErr = fmt.Errorf("permission denied")

	output := stripAnsi(m.viewSummary())

	if !strings.Contains(output, "Warning: failed to prune worktree metadata") {
		t.Error("should show prune failure warning")
	}
	if !strings.Contains(output, "permission denied") {
		t.Error("should include the prune error message")
	}
	if !strings.Contains(output, "Run `git worktree prune`") {
		t.Error("should say how to retry the prune")
	}
	if !strings.Contains(output, "1 worktree removed successfully") {
		t.Errorf("a failed prune must not change the removal verdict:\n%s", output)
	}
}

func TestWithMinProgressDuration_SetsField(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", nil, repo.ContextNoRepo,
		WithMinProgressDuration(500*time.Millisecond))

	if m.minProgressDuration != 500*time.Millisecond {
		t.Errorf("minProgressDuration = %v, want 500ms", m.minProgressDuration)
	}
}

func TestHoldOrAdvance_MinDurationNotElapsed_HoldsAndReturnsCmd(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.minProgressDuration = 10 * time.Second
	m.progressStartedAt = time.Now()
	m.progressToken = 1

	result, cmd := m.holdOrAdvance(summaryView)
	model := result.(Model)

	if model.view == summaryView {
		t.Error("should not transition immediately when min duration not elapsed")
	}
	if cmd == nil {
		t.Error("expected a tea.Tick cmd when holding")
	}
	if model.progressTargetView != summaryView {
		t.Errorf("progressTargetView = %d, want summaryView", model.progressTargetView)
	}
}

func TestHoldOrAdvance_MinDurationElapsed_StillSettles(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.minProgressDuration = 1 * time.Millisecond
	m.progressStartedAt = time.Now().Add(-100 * time.Millisecond) // hold long expired
	m.progressToken = 1

	result, cmd := m.holdOrAdvance(summaryView)
	model := result.(Model)

	if model.view == summaryView {
		t.Error("a flow that outlived the hold must not cut away mid-glide")
	}
	if cmd == nil {
		t.Error("expected a settle-floor tick so the bar visibly finishes")
	}
}

func TestHoldOrAdvance_NoEntryHold_StillSettles(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.minProgressDuration = 0
	m.progressToken = 1

	result, cmd := m.holdOrAdvance(summaryView)
	model := result.(Model)

	if model.view == summaryView {
		t.Error("the completion settle applies in every run mode: no instant cut")
	}
	if !model.progressSettling {
		t.Error("holdOrAdvance must begin settling")
	}
	if cmd == nil {
		t.Error("expected the hard-timeout probe cmd")
	}
}

func TestObserveSettle_AdvancesAfterBeat(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView
	m.progressTargetView = summaryView
	m.progressSettling = true
	m.progressSettlingSince = time.Now().Add(-time.Second)
	m.progressSettledAt = time.Now().Add(-progressSettleBeat - time.Millisecond)

	model, advanced := m.observeSettle(time.Now())
	if !advanced || !model.progressTransitionPending || model.view != progressView {
		t.Errorf("settled fill must schedule a renderer-safe transition, got view %d pending=%v", model.view, model.progressTransitionPending)
	}
	if model.progressSettling {
		t.Error("advancing must end the settling state")
	}
}

func TestObserveSettle_BeatNotElapsed_Holds(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView
	m.progressTargetView = summaryView
	m.progressSettling = true
	m.progressSettlingSince = time.Now()
	m.progressSettledAt = time.Now() // just settled: beat starts now

	model, advanced := m.observeSettle(time.Now())
	if advanced || model.view == summaryView {
		t.Error("the view must hold until the settled beat elapses")
	}
}

func TestObserveSettle_EntryHoldGates(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView
	m.progressTargetView = summaryView
	m.minProgressDuration = 10 * time.Second // playground entry hold, far from done
	m.progressStartedAt = time.Now()
	m.progressSettling = true
	m.progressSettlingSince = time.Now()
	m.progressSettledAt = time.Now().Add(-progressSettleBeat - time.Millisecond)

	model, advanced := m.observeSettle(time.Now())
	if advanced || model.view == summaryView {
		t.Error("playground entry hold must keep gating even after the beat")
	}
}

func TestObserveSettle_TimeoutForcesAdvance(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView
	m.progressTargetView = summaryView
	m.progressSettling = true
	m.progressSettlingSince = time.Now().Add(-progressSettleTimeout - time.Millisecond)
	// Never settled (settledAt zero): the hard timeout must still advance.

	model, advanced := m.observeSettle(time.Now())
	if !advanced || !model.progressTransitionPending || model.view != progressView {
		t.Error("the hard timeout must schedule a view that cannot settle")
	}
}

func TestSettleProbe_StaleToken_Ignored(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView
	m.progressToken = 5
	m.progressTargetView = summaryView
	m.progressSettling = true
	m.progressSettlingSince = time.Now().Add(-progressSettleTimeout - time.Millisecond)

	result, _ := m.Update(progressSettleProbeMsg{token: 3}) // stale token
	model := result.(Model)

	if model.view == summaryView {
		t.Error("stale probe token should not transition view")
	}
}

func TestSettleProbe_CorrectToken_RunsCheck(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView
	m.progressToken = 3
	m.progressTargetView = summaryView
	m.progressSettling = true
	m.progressSettlingSince = time.Now().Add(-progressSettleTimeout - time.Millisecond)

	result, cmd := m.Update(progressSettleProbeMsg{token: 3})
	model := result.(Model)

	if model.view != progressView || !model.progressTransitionPending || cmd == nil {
		t.Errorf("timeout probe must schedule refresh before transition, view=%d pending=%v cmd=%v", model.view, model.progressTransitionPending, cmd != nil)
	}
	completed, _ := model.Update(progressTransitionMsg{token: 3})
	if completed.(Model).view != summaryView {
		t.Errorf("post-refresh transition ended at %d, want summary", completed.(Model).view)
	}
}
