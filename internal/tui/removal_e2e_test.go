package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

// TestE2E_RemovalFlowChrome drives select -> confirm -> progress -> summary
// through direct model updates and asserts each screen renders the unified
// chrome (standard title, bar, hints, markers).
func TestE2E_RemovalFlowChrome(t *testing.T) {
	wts := []git.Worktree{
		{Path: "/work/a", Branch: "refs/heads/feature/a"},
		{Path: "/work/b", Branch: "refs/heads/feature/b"},
	}
	m := NewModel(wts, &stubRunner{responses: map[string]stubResponse{
		"/repo worktree prune": {output: ""},
	}}, "/repo")
	m.width, m.height = 100, 30
	m.remove.selected = map[string]bool{"/work/a": true, "/work/b": true}
	m.view = confirmView

	confirmScreen := stripANSI(m.viewConfirm())
	if !strings.Contains(confirmScreen, "sentei ─ Confirm deletion") || strings.Contains(confirmScreen, "╭") {
		t.Fatalf("confirm screen chrome wrong:\n%s", confirmScreen)
	}

	updated, _ := m.updateConfirm(keyRune('y'))
	m = updated.(Model)
	if m.view != progressView {
		t.Fatalf("expected progressView, got %d", m.view)
	}

	progressScreen := stripANSI(m.viewProgress())
	for _, want := range []string{"sentei ─ Removing worktrees", "Removing worktrees", "0/2", "░", "q quit"} {
		if !strings.Contains(progressScreen, want) {
			t.Fatalf("progress screen missing %q:\n%s", want, progressScreen)
		}
	}

	updated, _ = m.updateProgress(deletionsCompleteMsg{Result: worktree.DeletionResult{SuccessCount: 2}})
	m = updated.(Model)
	updated, _ = m.updateProgress(pruneCompleteMsg{})
	m = updated.(Model)
	updated, _ = m.updateProgress(cleanupCompleteMsg{})
	m = updated.(Model)
	updated, _ = m.updateProgress(removalEventsCompleteMsg{})
	m = updated.(Model)
	m = settleNow(t, m)
	if m.view != summaryView {
		t.Fatalf("expected summaryView, got %d", m.view)
	}

	summaryScreen := stripANSI(m.viewSummary())
	if !strings.Contains(summaryScreen, "sentei ─ Removal complete") || !strings.Contains(summaryScreen, "✦ 2 worktrees removed successfully") {
		t.Fatalf("summary screen chrome wrong:\n%s", summaryScreen)
	}
}

func TestE2E_ProgressViewsQuitOnKeys(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.remove.run = newRemovalRun(nil)

	for _, view := range []viewState{progressView, createProgressView, repoProgressView, integrationProgressView} {
		m.view = view
		for _, k := range []tea.KeyPressMsg{{Code: 'q', Text: "q"}, {Code: 'c', Mod: tea.ModCtrl}} {
			_, cmd := m.Update(k)
			if cmd == nil {
				t.Errorf("view %d: expected quit Cmd for %v", view, k)
				continue
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("view %d: expected tea.Quit for %v", view, k)
			}
		}
	}
}

func TestUpdateProgress_WindowSizeUpdatesWindowing(t *testing.T) {
	worktrees := make([]git.Worktree, 30)
	for i := range worktrees {
		worktrees[i] = git.Worktree{Path: "/w/" + stepName(i) + string(rune('0'+i/26)), Branch: "refs/heads/b" + stepName(i)}
	}
	m := NewModel(worktrees, nil, "/repo")
	m.width = 100
	m.remove.run = newRemovalRun(worktrees)
	m.remove.run.statuses[worktrees[0].Path] = statusRemoving
	for i, wt := range worktrees {
		stepID := progress.StepID(stepName(i))
		m.remove.run.events = append(m.remove.run.events, progress.Event{Phase: worktree.RemovalPhaseID, PhaseLabel: worktree.RemovalPhaseName, Step: stepID, StepLabel: wt.Branch, Status: progress.StepPending, Of: 2})
	}
	m.remove.run.events = append(m.remove.run.events,
		progress.Event{Phase: worktree.RemovalPhaseID, PhaseLabel: worktree.RemovalPhaseName, Close: true},
		progress.Event{Phase: worktree.RemovalPhaseID, Step: progress.StepID(stepName(0)), Status: progress.StepRunning, Checkpoint: 1, Of: 2},
	)
	m.view = progressView

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 16})
	m = updated.(Model)
	if !strings.Contains(stripANSI(m.viewProgress()), "showing") {
		t.Error("expected windowed step list with stat line at height 16")
	}

	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 80})
	m = updated.(Model)
	if strings.Contains(stripANSI(m.viewProgress()), "showing") {
		t.Error("expected full step list (no stat line) at height 80")
	}

	// Sanity: the run state itself is untouched by resizes.
	if m.remove.run.total() != 30 || m.remove.run.statuses[worktrees[0].Path] != statusRemoving {
		t.Error("resize must not mutate run state")
	}
}
