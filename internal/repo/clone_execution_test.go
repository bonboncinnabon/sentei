package repo

import (
	"errors"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestPreparedClone_FailureAndRollbackPolicy(t *testing.T) {
	base := prepareClone(&mock.Runner{}, CloneOptions{URL: "url", Location: "/tmp", Name: "project"})
	if len(base.operations) != 8 {
		t.Fatalf("operations = %d, want validation, clone, structure, worktree, tracking, rollback", len(base.operations))
	}
	for failedAt := 0; failedAt < 7; failedAt++ {
		t.Run(base.operations[failedAt].label, func(t *testing.T) {
			prepared := base
			prepared.operations = append([]cloneOperation(nil), base.operations...)
			for i := range prepared.operations {
				prepared.operations[i].run = func() (string, error) { return "", nil }
			}
			prepared.operations[failedAt].run = func() (string, error) { return "", errors.New("injected") }
			result := prepared.run(func(progress.Event) {})
			if result.Err != nil {
				t.Fatalf("Err = %v", result.Err)
			}
			failed := resultStepByID(t, result.Phases, prepared.operations[failedAt].phaseID, prepared.operations[failedAt].stepID)
			if failedAt == 6 {
				if failed.Status != progress.StepSkipped || result.WorktreePath == "" {
					t.Fatalf("tracking result=%#v worktree=%q", failed, result.WorktreePath)
				}
			} else if failed.Status != progress.StepFailed {
				t.Fatalf("failed operation = %#v", failed)
			}
			rollback := resultStepByID(t, result.Phases, cloneRollbackPhaseID, "remove-partial-checkout")
			wantRollback := progress.StepSkipped
			if failedAt >= 1 && failedAt <= 5 {
				wantRollback = progress.StepDone
			}
			if rollback.Status != wantRollback {
				t.Fatalf("rollback = %#v, want status %v", rollback, wantRollback)
			}
		})
	}
}

func TestPreparedClone_RollbackFailureIsSurfaced(t *testing.T) {
	prepared := prepareClone(&mock.Runner{}, CloneOptions{URL: "url", Location: "/tmp", Name: "project"})
	for i := range prepared.operations {
		prepared.operations[i].run = func() (string, error) { return "", nil }
	}
	prepared.operations[1].run = func() (string, error) { return "", errors.New("clone") }
	prepared.operations[7].run = func() (string, error) { return "", errors.New("rollback") }
	result := prepared.run(func(progress.Event) {})
	rollback := resultStepByID(t, result.Phases, cloneRollbackPhaseID, "remove-partial-checkout")
	if rollback.Status != progress.StepFailed || rollback.Error == nil {
		t.Fatalf("rollback = %#v", rollback)
	}
}

func TestPreparedClone_CallbackPanicPopulatesErr(t *testing.T) {
	want := errors.New("delivery")
	result := prepareClone(&mock.Runner{}, CloneOptions{URL: "url", Location: "/tmp", Name: "project"}).run(func(progress.Event) { panic(want) })
	if !errors.Is(result.Err, want) || len(result.Phases) != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestPreparedClone_ResultMatchesCompletedStream(t *testing.T) {
	prepared := prepareClone(&mock.Runner{}, CloneOptions{URL: "url", Location: "/tmp", Name: "project"})
	for i := range prepared.operations {
		prepared.operations[i].run = func() (string, error) { return "", nil }
	}
	var events []progress.Event
	result := prepared.run(func(event progress.Event) { events = append(events, event) })
	assertRepoStreamParity(t, events, result.Phases)
}

func assertRepoStreamParity(t *testing.T, events []progress.Event, phases []progress.Phase) {
	t.Helper()
	if err := progress.ValidateStream(events); err != nil {
		t.Fatalf("invalid completed stream: %v", err)
	}
	for _, phase := range phases {
		for _, step := range phase.Steps {
			last := progress.Event{}
			for _, event := range events {
				if event.Phase == phase.ID && event.Step == step.ID && !event.Close {
					last = event
				}
			}
			if last.Status != step.Status {
				t.Fatalf("%s/%s result=%v stream=%v", phase.ID, step.ID, step.Status, last.Status)
			}
		}
	}
}
