package worktree

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

func TestDeleteWorktrees_DeliveryFailurePopulatesErr(t *testing.T) {
	deliveryErr := errors.New("delivery failed")
	emissions := 0
	execution, err := progress.Start(progress.Plan{Phases: []progress.PlannedPhase{{
		ID: RemovalPhaseID, Steps: []progress.PlannedStep{{ID: "remove-a", Checkpoints: 2}},
	}}}, func(progress.Event) {
		emissions++
		if emissions > 2 {
			panic(deliveryErr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	result := DeleteWorktrees(execution, RemovalPhaseID, func(string) error { return nil }, []RemovalTarget{{
		Worktree: git.Worktree{Path: "/work/a"}, StepID: "remove-a",
	}}, 1)
	if result.Err == nil || !strings.Contains(result.Err.Error(), deliveryErr.Error()) {
		t.Fatalf("DeleteWorktrees() error = %v", result.Err)
	}
}

func TestDeleteWorktrees_UsesCallerExecutionAndLeavesChannelOpen(t *testing.T) {
	targets := []RemovalTarget{
		{Worktree: git.Worktree{Path: "/work/a"}, StepID: "remove-a"},
		{Worktree: git.Worktree{Path: "/work/b"}, StepID: "remove-b"},
	}
	events := make(chan progress.Event, 32)
	execution, err := progress.Start(progress.Plan{Phases: []progress.PlannedPhase{{
		ID: RemovalPhaseID, Label: RemovalPhaseName,
		Steps: []progress.PlannedStep{
			{ID: "remove-a", Label: "/work/a", Checkpoints: 2},
			{ID: "remove-b", Label: "/work/b", Checkpoints: 2},
		},
	}}}, func(event progress.Event) { events <- event })
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result := DeleteWorktrees(execution, RemovalPhaseID, func(string) error { return nil }, targets, 2)
	if result.Err != nil {
		t.Fatalf("DeleteWorktrees() error = %v", result.Err)
	}
	if result.SuccessCount != 2 || result.FailureCount != 0 {
		t.Fatalf("counts = (%d, %d), want (2, 0)", result.SuccessCount, result.FailureCount)
	}
	for i, target := range targets {
		if result.Outcomes[i].Path != target.Worktree.Path || !result.Outcomes[i].Success {
			t.Fatalf("outcome[%d] = %+v", i, result.Outcomes[i])
		}
	}

	// The caller still owns the channel and can continue using it.
	events <- progress.Event{Phase: "caller", Close: true}
	if err := execution.Finish("test complete"); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	close(events)

	var all []progress.Event
	for event := range events {
		all = append(all, event)
	}
	states := progress.Snapshot(all)
	if len(states) < 1 || states[0].Done != 2 || !states[0].Settled() {
		t.Fatalf("removal state = %+v", states)
	}
}

func TestDeleteWorktrees_JoinsWorkersBeforeReturning(t *testing.T) {
	targets := []RemovalTarget{
		{Worktree: git.Worktree{Path: "/work/a"}, StepID: "remove-a"},
		{Worktree: git.Worktree{Path: "/work/b"}, StepID: "remove-b"},
	}
	execution, err := progress.Start(progress.Plan{Phases: []progress.PlannedPhase{{
		ID: RemovalPhaseID,
		Steps: []progress.PlannedStep{
			{ID: "remove-a", Checkpoints: 2},
			{ID: "remove-b", Checkpoints: 2},
		},
	}}}, nil)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	var mu sync.Mutex
	completed := map[string]bool{}
	result := DeleteWorktrees(execution, RemovalPhaseID, func(path string) error {
		mu.Lock()
		completed[path] = true
		mu.Unlock()
		return nil
	}, targets, 2)
	if result.Err != nil {
		t.Fatalf("DeleteWorktrees() error = %v", result.Err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !completed["/work/a"] || !completed["/work/b"] {
		t.Fatalf("returned before workers completed: %v", completed)
	}
}
