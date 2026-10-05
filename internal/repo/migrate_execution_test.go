package repo

import (
	"errors"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestPrepareMigrate_FreezesOriginBeforeExecution(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[remote get-url origin]": {Output: "git@example/repo.git"},
	}}
	prepared := prepareMigrate(runner, runner, MigrateOptions{RepoPath: "/repo"})
	if prepared.err != nil {
		t.Fatal(prepared.err)
	}
	found := false
	for _, operation := range prepared.operations {
		found = found || operation.stepID == "restore-origin"
	}
	if !found {
		t.Fatal("actual origin did not freeze restore operation")
	}
	if len(runner.Calls) != 1 || runner.Calls[0] != "/repo:[remote get-url origin]" {
		t.Fatalf("preflight calls = %v", runner.Calls)
	}
}

func TestPrepareMigrate_DistinguishesNoOriginFromLookupFailure(t *testing.T) {
	t.Run("verified no origin", func(t *testing.T) {
		runner := &mock.Runner{Responses: map[string]mock.Response{
			"/repo:[remote get-url origin]": {Err: errors.New("git remote get-url origin: error: No such remote 'origin'")},
		}}
		prepared := prepareMigrate(runner, runner, MigrateOptions{RepoPath: "/repo"})
		if prepared.err != nil {
			t.Fatalf("err = %v", prepared.err)
		}
		for _, operation := range prepared.operations {
			if operation.stepID == "restore-origin" {
				t.Fatal("restore-origin declared without an origin")
			}
		}
	})

	t.Run("lookup infrastructure failure", func(t *testing.T) {
		lookupErr := errors.New("permission denied reading git config")
		runner := &mock.Runner{Responses: map[string]mock.Response{
			"/repo:[remote get-url origin]": {Err: lookupErr},
		}}
		prepared := prepareMigrate(runner, runner, MigrateOptions{RepoPath: "/repo"})
		result := prepared.run(func(progress.Event) {})
		if !errors.Is(result.Err, lookupErr) {
			t.Fatalf("Err = %v, want wrapped lookup failure", result.Err)
		}
		if len(result.Phases) != 0 || len(runner.Calls) != 1 {
			t.Fatalf("destructive work started: phases=%#v calls=%v", result.Phases, runner.Calls)
		}
	})
}

func TestPreparedMigrate_FailurePolicyAndBackupInformation(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{"/repo:[remote get-url origin]": {}}}
	base := prepareMigrate(runner, runner, MigrateOptions{RepoPath: "/repo"})
	for failedAt := range base.operations {
		t.Run(base.operations[failedAt].label, func(t *testing.T) {
			prepared := base
			backupPath := "/repo_backup_20260715_120000"
			prepared.backupPath = &backupPath
			prepared.operations = append([]migrateOperation(nil), base.operations...)
			for i := range prepared.operations {
				prepared.operations[i].run = func(*progress.Execution) (string, error) { return "", nil }
			}
			prepared.operations[failedAt].run = func(*progress.Execution) (string, error) { return "", errors.New("injected") }
			result := prepared.run(func(progress.Event) {})
			if result.Err != nil {
				t.Fatalf("Err = %v", result.Err)
			}
			failed := prepared.operations[failedAt]
			if step := resultStepByID(t, result.Phases, failed.phaseID, failed.stepID); step.Status != progress.StepFailed {
				t.Fatalf("failed result = %#v", step)
			}
			for _, later := range prepared.operations[failedAt+1:] {
				if step := resultStepByID(t, result.Phases, later.phaseID, later.stepID); step.Status != progress.StepSkipped {
					t.Fatalf("later %s = %#v", later.label, step)
				}
			}
			backupExpected := failedAt > prepared.backupCopyIndex
			if (result.BackupPath != "") != backupExpected {
				t.Fatalf("BackupPath=%q failedAt=%d backupIndex=%d", result.BackupPath, failedAt, prepared.backupCopyIndex)
			}
		})
	}
}

func TestPreparedMigrate_CallbackPanicPopulatesErr(t *testing.T) {
	want := errors.New("delivery")
	runner := &mock.Runner{Responses: map[string]mock.Response{"/repo:[remote get-url origin]": {}}}
	result := prepareMigrate(runner, runner, MigrateOptions{RepoPath: "/repo"}).run(func(progress.Event) { panic(want) })
	if !errors.Is(result.Err, want) || len(result.Phases) != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestPreparedMigrate_BackupPathSurvivesDoneDeliveryPanic(t *testing.T) {
	want := errors.New("delivery")
	runner := &mock.Runner{Responses: map[string]mock.Response{"/repo:[remote get-url origin]": {}}}
	prepared := prepareMigrate(runner, runner, MigrateOptions{RepoPath: "/repo"})
	*prepared.backupPath = "/repo_backup_20260715_120000"
	for i := range prepared.operations {
		prepared.operations[i].run = func(*progress.Execution) (string, error) { return "", nil }
	}
	result := prepared.run(func(event progress.Event) {
		if event.Phase == "migrate:backup" && event.Step == "copy" && event.Status == progress.StepDone {
			panic(want)
		}
	})
	if !errors.Is(result.Err, want) {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.BackupPath != *prepared.backupPath {
		t.Fatalf("BackupPath = %q, want %q", result.BackupPath, *prepared.backupPath)
	}
}

func TestPreparedMigrate_ResultMatchesCompletedStream(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{"/repo:[remote get-url origin]": {}}}
	prepared := prepareMigrate(runner, runner, MigrateOptions{RepoPath: "/repo"})
	for i := range prepared.operations {
		prepared.operations[i].run = func(*progress.Execution) (string, error) { return "", nil }
	}
	var events []progress.Event
	result := prepared.run(func(event progress.Event) { events = append(events, event) })
	assertRepoStreamParity(t, events, result.Phases)
}
