package repo

import (
	"errors"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

// stubbedClone is the declared clone flow with every step and undo replaced
// by a no-op, so a test can fail one step and observe the policy around it.
func stubbedClone() (flow, func([]progress.Phase, error) CloneResult) {
	clone, finish := prepareClone(&mock.Runner{}, CloneOptions{URL: "url", Location: "/tmp", Name: "project"})
	for i := range clone.steps {
		clone.steps[i].run = func() (string, error) { return "", nil }
		if clone.steps[i].undo != nil {
			clone.steps[i].undo = func() error { return nil }
		}
	}
	return clone, finish
}

func TestPrepareClone_FailureAndRollbackPolicy(t *testing.T) {
	declared, _ := stubbedClone()
	if len(declared.steps) != 7 || declared.rollback == nil {
		t.Fatalf("steps = %d, want validation, clone, two structure steps, three worktree steps, plus a rollback", len(declared.steps))
	}
	for failedAt, failing := range declared.steps {
		t.Run(failing.label, func(t *testing.T) {
			clone, finish := stubbedClone()
			clone.steps[failedAt].run = func() (string, error) { return "", errors.New("injected") }

			result := finish(clone.run(nil))

			if result.Err != nil {
				t.Fatalf("Err = %v", result.Err)
			}
			failed := stepResult(result.Phases, failing.phaseID, failing.id)
			// Everything from the bare clone on leaves a partial checkout to
			// remove, except the tracking step: without tracking the clone
			// is still usable.
			wantFailed, wantRollback := progress.StepFailed, progress.StepSkipped
			switch {
			case failing.id == "tracking":
				wantFailed = progress.StepSkipped
				if result.WorktreePath == "" {
					t.Error("a clone without tracking still has its worktree")
				}
			case failedAt >= 1:
				wantRollback = progress.StepDone
			}
			if failed.Status != wantFailed {
				t.Errorf("failed step = %#v, want status %v", failed, wantFailed)
			}
			if rollback := stepResult(result.Phases, cloneRollbackPhaseID, "remove-partial-checkout"); rollback.Status != wantRollback {
				t.Errorf("rollback = %#v, want status %v", rollback, wantRollback)
			}
		})
	}
}

func TestPrepareClone_RollbackFailureIsSurfaced(t *testing.T) {
	clone, finish := stubbedClone()
	clone.steps[1].run = func() (string, error) { return "", errors.New("clone") }
	clone.steps[1].undo = func() error { return errors.New("rollback") }

	result := finish(clone.run(nil))

	rollback := stepResult(result.Phases, cloneRollbackPhaseID, "remove-partial-checkout")
	if rollback.Status != progress.StepFailed || rollback.Error == nil || !result.HasFailures() {
		t.Fatalf("rollback = %#v", rollback)
	}
}

func TestClone_CallbackPanicPopulatesErr(t *testing.T) {
	want := errors.New("delivery")
	result := Clone(&mock.Runner{}, CloneOptions{URL: "url", Location: "/tmp", Name: "project"}, func(progress.Event) { panic(want) })
	if !errors.Is(result.Err, want) || len(result.Phases) != 0 {
		t.Fatalf("result = %#v", result)
	}
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
