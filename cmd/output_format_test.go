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
	tests := []struct {
		name  string
		event progress.Event
		want  []string
	}{
		{"running", progress.Event{Phase: "clone", Step: "fetch", Status: progress.StepRunning}, []string{"→", "[clone]", "fetch"}},
		{"done", progress.Event{Phase: "clone", Step: "fetch", Status: progress.StepDone}, []string{"✓", "[clone]", "fetch"}},
		{"done with message", progress.Event{Phase: "clone", Step: "fetch", Status: progress.StepDone, Message: "fast"}, []string{"✓", "(fast)"}},
		{"failed", progress.Event{Phase: "clone", Step: "fetch", Status: progress.StepFailed, Error: errors.New("boom")}, []string{"✗", "boom"}},
		{"skipped", progress.Event{Phase: "clone", Step: "fetch", Status: progress.StepSkipped}, []string{"⊘", "fetch"}},
		{"skipped with message", progress.Event{Phase: "clone", Step: "fetch", Status: progress.StepSkipped, Message: "no-op"}, []string{"⊘", "(no-op)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() { printCloneEvent(tt.event) })
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output %q missing %q", out, want)
				}
			}
		})
	}
}

func TestPrintCreateEvent(t *testing.T) {
	tests := []struct {
		name  string
		event progress.Event
		want  []string
	}{
		{"running", progress.Event{Phase: "create", Step: "branch", Status: progress.StepRunning}, []string{"→", "create", "branch"}},
		{"done", progress.Event{Step: "branch", Status: progress.StepDone}, []string{"✓", "branch"}},
		{"done with message", progress.Event{Step: "branch", Status: progress.StepDone, Message: "created"}, []string{"✓", "— created"}},
		{"failed", progress.Event{Step: "branch", Status: progress.StepFailed, Error: errors.New("boom")}, []string{"✗", "— boom"}},
		{"failed without error", progress.Event{Step: "branch", Status: progress.StepFailed}, []string{"✗", "branch"}},
		{"skipped", progress.Event{Step: "branch", Status: progress.StepSkipped}, []string{"branch (skipped)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() { printCreateEvent(tt.event) })
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output %q missing %q", out, want)
				}
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
