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
	const id = "create-worktree:feature/x"
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
			// Step identities match events; only labels are for people.
			if strings.Contains(out, id) {
				t.Errorf("output %q leaks the step identity %q", out, id)
			}
		})
	}
}

func TestPrintMigrateEvent(t *testing.T) {
	const phase, step = "migrate:convert", "move-git"
	event := func(status progress.StepStatus) progress.Event {
		return progress.Event{Phase: phase, PhaseLabel: "Convert", Step: step, StepLabel: "Move .git to .bare", Status: status}
	}
	failed := event(progress.StepFailed)
	failed.Error = errors.New("boom")
	done := event(progress.StepDone)
	done.Message = "ok"
	tests := []struct {
		name  string
		event progress.Event
		want  []string
	}{
		{"running", event(progress.StepRunning), []string{"→", "Convert: Move .git to .bare"}},
		{"done", event(progress.StepDone), []string{"✓", "Move .git to .bare"}},
		{"done with message", done, []string{"✓", "Move .git to .bare", "ok"}},
		{"failed", failed, []string{"✗", "Move .git to .bare", "boom"}},
		// After a refusal every later step is skipped; listing them is noise.
		{"skipped", event(progress.StepSkipped), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() { printMigrateEvent(tt.event) })
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output %q missing %q", out, want)
				}
			}
			if tt.want == nil && out != "" {
				t.Errorf("output %q, want nothing", out)
			}
			// Phase and step identities are for matching events, not for people.
			if strings.Contains(out, phase) || strings.Contains(out, step) {
				t.Errorf("output %q leaks an identity", out)
			}
		})
	}
}
