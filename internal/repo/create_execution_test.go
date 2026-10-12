package repo

import (
	"errors"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestPrepareCreate_FailurePolicyAtEveryDeclaredStep(t *testing.T) {
	opts := CreateOptions{Name: "project", Location: "/tmp", PublishGitHub: true, Visibility: "private"}
	declared, _ := prepareCreate(&mock.Runner{}, &mockGhRunner{}, opts)
	if len(declared.steps) != 11 {
		t.Fatalf("steps = %d, want six local plus five GitHub", len(declared.steps))
	}
	for failedAt, failing := range declared.steps {
		t.Run(failing.label, func(t *testing.T) {
			create, finish := prepareCreate(&mock.Runner{}, &mockGhRunner{}, opts)
			for i := range create.steps {
				create.steps[i].run = func() (string, error) { return "", nil }
			}
			create.steps[failedAt].run = func() (string, error) { return "", errors.New("injected") }

			result := finish(create.run(nil))

			if result.Err != nil {
				t.Fatalf("Err = %v, a step failure must remain a step failure", result.Err)
			}
			for i, declaredStep := range create.steps {
				want := progress.StepDone
				switch {
				case i == failedAt:
					want = progress.StepFailed
				case i > failedAt:
					want = progress.StepSkipped
				}
				if got := stepResult(result.Phases, declaredStep.phaseID, declaredStep.id); got.Status != want {
					t.Fatalf("%s status = %v, want %v", declaredStep.label, got.Status, want)
				}
			}
			if hard, _ := result.SetupFailed(); hard != (failedAt < 6) {
				t.Errorf("SetupFailed() = %v with a failure at step %d", hard, failedAt)
			}
			if failedAt >= 6 && result.WorktreePath == "" {
				t.Fatal("GitHub failure discarded usable local worktree")
			}
			if result.GitHubURL != "" {
				t.Errorf("GitHubURL = %q for a create that did not publish", result.GitHubURL)
			}
		})
	}
}

func TestCreate_CallbackPanicPopulatesErr(t *testing.T) {
	want := errors.New("delivery")
	result := CreateWithGh(&mock.Runner{}, &mockGhRunner{}, CreateOptions{Name: "project", Location: "/tmp"}, func(progress.Event) { panic(want) })
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
