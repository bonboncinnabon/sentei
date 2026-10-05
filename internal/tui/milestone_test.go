package tui

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/state"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func TestCrossedPowerOfTen(t *testing.T) {
	cases := []struct{ before, after, want int }{
		{0, 3, 0}, {7, 9, 0}, {9, 10, 10}, {8, 12, 10},
		{10, 11, 0}, {95, 105, 100}, {5, 1500, 1000}, {99, 100, 100},
	}
	for _, tc := range cases {
		if got := crossedPowerOfTen(tc.before, tc.after); got != tc.want {
			t.Errorf("crossedPowerOfTen(%d, %d) = %d, want %d", tc.before, tc.after, got, tc.want)
		}
	}
}

func TestRecordRemovals_PersistsAndReportsMilestone(t *testing.T) {
	dir := t.TempDir()
	if err := state.Save(dir, &state.State{LifetimeRemoved: 8}); err != nil {
		t.Fatal(err)
	}

	msg := recordRemovals(dir, 4)()
	got, ok := msg.(milestoneMsg)
	if !ok || got.Crossed != 10 {
		t.Fatalf("expected milestone 10, got %#v", msg)
	}
	s, _ := state.Load(dir)
	if s.LifetimeRemoved != 12 {
		t.Errorf("lifetime = %d, want 12", s.LifetimeRemoved)
	}
	if s.Integrations == nil && len(s.Integrations) != 0 {
		t.Error("unrelated state must survive")
	}
}

func TestRecordRemovals_ZeroIsSilent(t *testing.T) {
	dir := t.TempDir()
	msg := recordRemovals(dir, 0)()
	if got := msg.(milestoneMsg); got.Crossed != 0 {
		t.Errorf("zero removals must not whisper, got %d", got.Crossed)
	}
	if s, _ := state.Load(dir); s.LifetimeRemoved != 0 {
		t.Error("zero removals must not write")
	}
}

func TestSummary_WhispersAtMilestone(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.width = 80
	m.remove.run = newRemovalRun(nil)
	m.remove.run.result = worktree.DeletionResult{SuccessCount: 2}
	m.view = summaryView

	if got := stripAnsi(m.viewSummary()); strings.Contains(got, "pruned\n") && strings.Contains(got, "that was your") {
		t.Errorf("no whisper without a milestone:\n%s", got)
	}

	m.remove.milestone = 100
	got := stripAnsi(m.viewSummary())
	if !strings.Contains(got, "that was your 100th worktree, pruned") {
		t.Errorf("expected milestone whisper:\n%s", got)
	}
}

func TestMilestoneMsg_HandledGlobally(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = summaryView // the hold may already have advanced the view

	updated, _ := m.Update(milestoneMsg{Crossed: 10})
	if updated.(Model).remove.milestone != 10 {
		t.Error("milestoneMsg must land regardless of the active view")
	}
}
