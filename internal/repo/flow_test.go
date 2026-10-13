package repo

import (
	"errors"
	"slices"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

// recordingFlow is three steps and a rollback. Each step and undo appends its
// name to the returned log, so a test can assert what ran and in what order.
func recordingFlow() (*flow, *[]string) {
	var log []string
	f := &flow{name: "test flow"}
	for _, id := range []string{"one", "two", "three"} {
		f.add("work", "Work", step{
			id: id, label: "Step " + id,
			run:  func() (string, error) { log = append(log, "run "+id); return "", nil },
			undo: func() error { log = append(log, "undo "+id); return nil },
		})
	}
	f.addRollback("rollback", "undo", "Undo")
	return f, &log
}

func failing(log *[]string, name string) progress.StepFunc {
	return func() (string, error) {
		*log = append(*log, "run "+name)
		return "", errors.New("injected")
	}
}

func statuses(t *testing.T, phases []progress.Phase) map[progress.StepID]progress.StepStatus {
	t.Helper()
	got := map[progress.StepID]progress.StepStatus{}
	for _, phase := range phases {
		for _, result := range phase.Steps {
			got[result.ID] = result.Status
		}
	}
	return got
}

func TestFlow_SuccessRunsEveryStepAndSkipsRollback(t *testing.T) {
	f, log := recordingFlow()
	var events []progress.Event

	phases, err := f.run(func(event progress.Event) { events = append(events, event) })

	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"run one", "run two", "run three"}; !slices.Equal(*log, want) {
		t.Errorf("ran %v, want %v", *log, want)
	}
	got := statuses(t, phases)
	for _, id := range []string{"one", "two", "three"} {
		if got[id] != progress.StepDone {
			t.Errorf("step %s = %v, want done", id, got[id])
		}
	}
	if got["undo"] != progress.StepSkipped {
		t.Errorf("rollback = %v, want skipped", got["undo"])
	}
	assertRepoStreamParity(t, events, phases)
}

func TestFlow_FailureSkipsTheRestAndUndoesStartedStepsNewestFirst(t *testing.T) {
	f, log := recordingFlow()
	f.steps[1].run = failing(log, "two")
	var events []progress.Event

	phases, err := f.run(func(event progress.Event) { events = append(events, event) })

	if err != nil {
		t.Fatalf("a step failure belongs in the phases, not the error: %v", err)
	}
	// Step two failed part-way, so its own undo runs too. Step three never
	// started and must not be undone.
	if want := []string{"run one", "run two", "undo two", "undo one"}; !slices.Equal(*log, want) {
		t.Errorf("ran %v, want %v", *log, want)
	}
	got := statuses(t, phases)
	want := map[progress.StepID]progress.StepStatus{
		"one": progress.StepDone, "two": progress.StepFailed, "three": progress.StepSkipped, "undo": progress.StepDone,
	}
	for id, status := range want {
		if got[id] != status {
			t.Errorf("step %s = %v, want %v", id, got[id], status)
		}
	}
	if blocked := stepResult(phases, "work", "three"); blocked.Message != "blocked by Step two" {
		t.Errorf("skip reason = %q", blocked.Message)
	}
	assertRepoStreamParity(t, events, phases)
}

func TestFlow_FailureBeforeAnyUndoableStepSkipsRollback(t *testing.T) {
	f, log := recordingFlow()
	f.steps[0].undo = nil
	f.steps[0].run = failing(log, "one")

	phases, err := f.run(nil)

	if err != nil {
		t.Fatal(err)
	}
	if rollback := stepResult(phases, "rollback", "undo"); rollback.Status != progress.StepSkipped {
		t.Errorf("rollback = %#v, want skipped: nothing had started", rollback)
	}
}

func TestFlow_FailedUndoFailsTheRollbackStepAndKeepsUndoing(t *testing.T) {
	f, log := recordingFlow()
	f.steps[2].run = failing(log, "three")
	stuck := errors.New("stuck")
	f.steps[1].undo = func() error { *log = append(*log, "undo two"); return stuck }

	phases, err := f.run(nil)

	if err != nil {
		t.Fatal(err)
	}
	if rollback := stepResult(phases, "rollback", "undo"); rollback.Status != progress.StepFailed || !errors.Is(rollback.Error, stuck) {
		t.Errorf("rollback = %#v, want failed with the undo error", rollback)
	}
	if !slices.Contains(*log, "undo one") {
		t.Errorf("a stuck undo stranded the earlier ones: %v", *log)
	}
}

func TestFlow_OptionalStepFailureIsSkippedAndDoesNotBlock(t *testing.T) {
	f, log := recordingFlow()
	f.steps[1].optional = true
	f.steps[1].run = failing(log, "two")
	var events []progress.Event

	phases, err := f.run(func(event progress.Event) { events = append(events, event) })

	if err != nil {
		t.Fatal(err)
	}
	got := statuses(t, phases)
	if got["two"] != progress.StepSkipped || got["three"] != progress.StepDone || got["undo"] != progress.StepSkipped {
		t.Errorf("statuses = %v, want the optional step skipped, the next done, no rollback", got)
	}
	if skipped := stepResult(phases, "work", "two"); skipped.Message != "injected" {
		t.Errorf("skip reason = %q, want the step's error", skipped.Message)
	}
	assertRepoStreamParity(t, events, phases)
}

// A consumer that panics breaks the event stream. Steps can no longer be
// reported, but whatever already ran still has to be undone.
func TestFlow_BrokenEventStreamStillUndoes(t *testing.T) {
	f, log := recordingFlow()
	delivery := errors.New("delivery")

	_, err := f.run(func(event progress.Event) {
		if event.Step == "two" && event.Status == progress.StepDone {
			panic(delivery)
		}
	})

	if !errors.Is(err, delivery) {
		t.Fatalf("err = %v, want the delivery failure", err)
	}
	if want := []string{"run one", "run two", "undo two", "undo one"}; !slices.Equal(*log, want) {
		t.Errorf("ran %v, want %v", *log, want)
	}
}

func TestFlow_PanicOnFirstEventRunsNothing(t *testing.T) {
	f, log := recordingFlow()
	delivery := errors.New("delivery")

	phases, err := f.run(func(progress.Event) { panic(delivery) })

	if !errors.Is(err, delivery) || len(phases) != 0 || len(*log) != 0 {
		t.Fatalf("phases=%v err=%v ran=%v, want only the start error", phases, err, *log)
	}
}
