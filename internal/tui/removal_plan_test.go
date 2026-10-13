package tui

import (
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

// Spec "Parallel removal moves the bar at start": started-but-unfinished
// removals reach their first checkpoint while headers still count 0 done.
func TestBuildRemovalPhases_StartCheckpointsMoveTheBar(t *testing.T) {
	wts := []git.Worktree{{Path: "/wt/a"}, {Path: "/wt/b"}, {Path: "/wt/c"}}
	m := NewModel(wts, nil, "/repo")
	m.remove.run = removalRun{}
	for i := range wts {
		stepID := progress.StepID(string(rune('a' + i)))
		m.remove.run.events = append(m.remove.run.events,
			progress.Event{Phase: worktree.RemovalPhaseID, PhaseLabel: worktree.RemovalPhaseName, Step: stepID, Status: progress.StepPending, Of: 2},
			progress.Event{Phase: worktree.RemovalPhaseID, Step: stepID, Status: progress.StepRunning, Checkpoint: 1, Of: 2},
		)
	}
	m.remove.run.events = append(m.remove.run.events, progress.Event{Phase: worktree.RemovalPhaseID, PhaseLabel: worktree.RemovalPhaseName, Close: true})

	phases := progress.Snapshot(m.remove.run.events)
	var removing progress.PhaseState
	for _, p := range phases {
		if p.Name == "Removing worktrees" {
			removing = p
		}
	}
	if removing.Done != 0 {
		t.Fatalf("headers must count 0 done while all removals run, got %d", removing.Done)
	}
	reached, declared := progress.CheckpointProgress([]progress.PhaseState{removing})
	if reached != 3 || declared != 6 {
		t.Errorf("checkpoints = %d/%d, want 3/6 (start credit for three parallel removals)", reached, declared)
	}
}
