package repo

import (
	"errors"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestPreparedCreate_FailurePolicyAtEveryDeclaredOperation(t *testing.T) {
	prepared := prepareCreate(&mock.Runner{}, &mockGhRunner{}, CreateOptions{
		Name: "project", Location: "/tmp", PublishGitHub: true, Visibility: "private",
	})
	if len(prepared.operations) != 11 {
		t.Fatalf("operations = %d, want six local plus five GitHub", len(prepared.operations))
	}
	for failedAt := range prepared.operations {
		t.Run(prepared.operations[failedAt].label, func(t *testing.T) {
			candidate := prepared
			candidate.operations = append([]createOperation(nil), prepared.operations...)
			for i := range candidate.operations {
				candidate.operations[i].run = func() (string, error) { return "", nil }
			}
			candidate.operations[failedAt].run = func() (string, error) { return "", errors.New("injected") }
			result := candidate.run(func(progress.Event) {})
			if result.Err != nil {
				t.Fatalf("Err = %v, operation failure must remain a step failure", result.Err)
			}
			for i, op := range candidate.operations {
				step := resultStepByID(t, result.Phases, op.phaseID, op.stepID)
				want := progress.StepDone
				switch {
				case i == failedAt:
					want = progress.StepFailed
				case i > failedAt && op.phaseID == candidate.operations[failedAt].phaseID:
					want = progress.StepSkipped
				case i > failedAt && candidate.operations[failedAt].phaseID == createSetupPhaseID:
					want = progress.StepSkipped
				}
				if step.Status != want {
					t.Fatalf("%s status = %v, want %v", op.label, step.Status, want)
				}
			}
			if failedAt >= 6 && result.WorktreePath == "" {
				t.Fatal("GitHub failure discarded usable local worktree")
			}
		})
	}
}

func TestPreparedCreate_CallbackPanicPopulatesErr(t *testing.T) {
	want := errors.New("delivery")
	prepared := prepareCreate(&mock.Runner{}, &mockGhRunner{}, CreateOptions{Name: "project", Location: "/tmp"})
	result := prepared.run(func(progress.Event) { panic(want) })
	if !errors.Is(result.Err, want) {
		t.Fatalf("Err = %v, want wrapped callback failure", result.Err)
	}
	if len(result.Phases) != 0 {
		t.Fatalf("Phases = %#v, want only Execution projection", result.Phases)
	}
}

func TestCreateResult_ContractErrorIsHardFailure(t *testing.T) {
	want := errors.New("delivery")
	result := CreateResult{Err: want}
	if !result.HasFailures() {
		t.Fatal("contract error must fail the result")
	}
	failed, err := result.SetupFailed()
	if !failed || !errors.Is(err, want) {
		t.Fatalf("SetupFailed() = %v, %v; want hard contract failure", failed, err)
	}
}

func TestPreparedCreate_ResultMatchesCompletedStream(t *testing.T) {
	prepared := prepareCreate(&mock.Runner{}, &mockGhRunner{}, CreateOptions{Name: "project", Location: "/tmp", PublishGitHub: true})
	for i := range prepared.operations {
		prepared.operations[i].run = func() (string, error) { return "", nil }
	}
	var events []progress.Event
	result := prepared.run(func(event progress.Event) { events = append(events, event) })
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if err := progress.ValidateStream(events); err != nil {
		t.Fatalf("invalid completed stream: %v", err)
	}
	for _, phase := range result.Phases {
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

func resultStepByID(t *testing.T, phases []progress.Phase, phaseID progress.PhaseID, stepID progress.StepID) progress.StepResult {
	t.Helper()
	for _, phase := range phases {
		if phase.ID != phaseID {
			continue
		}
		for _, step := range phase.Steps {
			if step.ID == stepID {
				return step
			}
		}
	}
	t.Fatalf("missing result %s/%s", phaseID, stepID)
	return progress.StepResult{}
}
