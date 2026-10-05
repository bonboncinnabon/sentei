package tui

import (
	"errors"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestIntegrationPreparedApplyUsesStrictFixedStream(t *testing.T) {
	worktrees := []string{"/wt/feat-1", "/wt/feat-2"}
	alpha := integration.Integration{
		Name: "alpha", Detect: integration.DetectSpec{Command: "alpha detect"},
		Install: integration.InstallSpec{Command: "alpha install"},
		Setup:   integration.SetupSpec{Command: "alpha setup {path}", WorkingDir: "worktree"},
	}
	beta := integration.Integration{
		Name: "beta", Detect: integration.DetectSpec{Command: "beta detect"},
		Install: integration.InstallSpec{Command: "beta install"},
		Setup:   integration.SetupSpec{Command: "beta setup {path}", WorkingDir: "worktree"},
	}
	shell := &mock.Runner{Responses: map[string]mock.Response{
		"/wt/feat-1:shell[alpha detect]":             {Output: "installed"},
		"/wt/feat-1:shell[beta detect]":              {Err: errors.New("missing")},
		"/wt/feat-1:shell[beta install]":             {Output: "installed"},
		"/wt/feat-1:shell[alpha setup '/wt/feat-1']": {Output: "ok"},
		"/wt/feat-2:shell[alpha setup '/wt/feat-2']": {Output: "ok"},
		"/wt/feat-1:shell[beta setup '/wt/feat-1']":  {Output: "ok"},
		"/wt/feat-2:shell[beta setup '/wt/feat-2']":  {Output: "ok"},
	}}
	prepared, err := integration.PrepareApply(shell, "/repo", worktrees[0], []integration.Integration{alpha, beta}, nil, worktrees)
	if err != nil {
		t.Fatal(err)
	}
	var events []progress.Event
	if _, err := prepared.Run(shell, func(event progress.Event) { events = append(events, event) }); err != nil {
		t.Fatal(err)
	}
	if err := progress.ValidateStream(events); err != nil {
		t.Fatalf("strict validation failed: %v", err)
	}

	declarationEnd := 0
	for i, event := range events {
		if event.Close {
			declarationEnd = i + 1
		}
	}
	_, fixedTotal := progress.CheckpointProgress(progress.Snapshot(events[:declarationEnd]))
	for prefix := declarationEnd; prefix <= len(events); prefix++ {
		_, total := progress.CheckpointProgress(progress.Snapshot(events[:prefix]))
		if total != fixedTotal {
			t.Fatalf("prefix %d total = %d, want %d", prefix, total, fixedTotal)
		}
	}

	m := NewModel(nil, nil, "/repo")
	m.integ.events = events
	phases := m.buildIntegrationPhases()
	for _, phase := range phases {
		if phase.Total > 0 && !phase.Settled() {
			t.Fatalf("phase did not settle: %#v", phase)
		}
	}
}
