package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

func TestPrintEvent(t *testing.T) {
	tests := []struct {
		name  string
		event cleanup.Event
		want  []string
	}{
		{"step", cleanup.Event{Level: cleanup.LevelStep, Message: "Checking refs"}, []string{"→", "Checking refs"}},
		{"info", cleanup.Event{Level: cleanup.LevelInfo, Message: "Pruned 2 refs"}, []string{"✓", "Pruned 2 refs"}},
		{"warn", cleanup.Event{Level: cleanup.LevelWarn, Message: "skipped branch"}, []string{"⚠", "skipped branch"}},
		{"detail", cleanup.Event{Level: cleanup.LevelDetail, Message: "3 branches remain"}, []string{"3 branches remain"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() { printEvent(tt.event) })
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output %q missing %q", out, want)
				}
			}
		})
	}
}

func TestPrintCloneEvent(t *testing.T) {
	event := func(status progress.StepStatus, message string, err error) progress.Event {
		return progress.Event{
			Phase: "clone:bare", PhaseLabel: "Clone",
			Step: "bare-repository", StepLabel: "Clone bare repository",
			Status: status, Message: message, Error: err,
		}
	}
	tests := []struct {
		name  string
		event progress.Event
		want  []string
	}{
		{"running", event(progress.StepRunning, "", nil), []string{"→", "[Clone]", "Clone bare repository"}},
		{"done", event(progress.StepDone, "", nil), []string{"✓", "[Clone]", "Clone bare repository"}},
		{"done with message", event(progress.StepDone, "fast", nil), []string{"✓", "(fast)"}},
		{"failed", event(progress.StepFailed, "", errors.New("boom")), []string{"✗", "Clone bare repository: boom"}},
		{"skipped", event(progress.StepSkipped, "", nil), []string{"⊘", "Clone bare repository"}},
		{"skipped with message", event(progress.StepSkipped, "no-op", nil), []string{"⊘", "(no-op)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() { printCloneEvent(tt.event) })
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output %q missing %q", out, want)
				}
			}
			// Identities are for matching events; only labels are for people.
			for _, id := range []string{"clone:bare", "bare-repository"} {
				if strings.Contains(out, id) {
					t.Errorf("output %q leaks the identity %q", out, id)
				}
			}
		})
	}
}

func TestPrintCreateEvent(t *testing.T) {
	const id = "create-worktree:217d2bf51f16e9dc"
	tests := []struct {
		name  string
		event progress.Event
		want  []string
	}{
		{"running", progress.Event{Phase: "setup", PhaseLabel: "Setup", Step: id, StepLabel: "branch", Status: progress.StepRunning}, []string{"→", "Setup", "branch"}},
		{"done", progress.Event{Step: id, StepLabel: "branch", Status: progress.StepDone}, []string{"✓", "branch"}},
		{"done with message", progress.Event{Step: id, StepLabel: "branch", Status: progress.StepDone, Message: "created"}, []string{"✓", "— created"}},
		{"failed", progress.Event{Step: id, StepLabel: "branch", Status: progress.StepFailed, Error: errors.New("boom")}, []string{"✗", "— boom"}},
		{"failed without error", progress.Event{Step: id, StepLabel: "branch", Status: progress.StepFailed}, []string{"✗", "branch"}},
		{"skipped", progress.Event{Step: id, StepLabel: "branch", Status: progress.StepSkipped}, []string{"branch (skipped)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() { printCreateEvent(tt.event) })
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output %q missing %q", out, want)
				}
			}
			// Step identities are opaque hashes; only labels are for people.
			if strings.Contains(out, id) {
				t.Errorf("output %q leaks the step identity %q", out, id)
			}
		})
	}
}

func TestPrintMigrateEvent(t *testing.T) {
	tests := []struct {
		name  string
		event progress.Event
		want  []string
	}{
		{"running", progress.Event{Phase: "backup", Step: "copy", Status: progress.StepRunning}, []string{"→", "[backup]", "copy"}},
		{"running with message", progress.Event{Phase: "backup", Step: "copy", Status: progress.StepRunning, Message: "1.2MB"}, []string{"→", "— 1.2MB"}},
		{"done", progress.Event{Phase: "backup", Step: "copy", Status: progress.StepDone}, []string{"✓", "[backup]", "copy"}},
		{"done with message", progress.Event{Phase: "backup", Step: "copy", Status: progress.StepDone, Message: "ok"}, []string{"✓", "(ok)"}},
		{"failed", progress.Event{Phase: "backup", Step: "copy", Status: progress.StepFailed, Error: errors.New("boom")}, []string{"✗", "boom"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() { printMigrateEvent(tt.event) })
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output %q missing %q", out, want)
				}
			}
		})
	}
}
