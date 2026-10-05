package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

func collectEvents(ch <-chan progress.Event) []progress.Event {
	var events []progress.Event
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func deleteWorktreesForTest(t *testing.T, remover func(string) error, worktrees []git.Worktree, maxConcurrency int, events chan progress.Event) DeletionResult {
	t.Helper()
	if len(worktrees) == 0 {
		result := DeleteWorktrees(nil, RemovalPhaseID, remover, nil, maxConcurrency)
		close(events)
		return result
	}
	targets := make([]RemovalTarget, len(worktrees))
	steps := make([]progress.PlannedStep, len(worktrees))
	for i, wt := range worktrees {
		stepID := progress.StepID(fmt.Sprintf("remove-%d", i))
		targets[i] = RemovalTarget{Worktree: wt, StepID: stepID}
		steps[i] = progress.PlannedStep{ID: stepID, Label: wt.Path, Checkpoints: 2}
	}
	execution, err := progress.Start(progress.Plan{Phases: []progress.PlannedPhase{{
		ID: RemovalPhaseID, Label: RemovalPhaseName, Steps: steps,
	}}}, func(event progress.Event) { events <- event })
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	result := DeleteWorktrees(execution, RemovalPhaseID, remover, targets, maxConcurrency)
	if finishErr := execution.Finish("test complete"); finishErr != nil {
		t.Fatalf("Finish() error = %v", finishErr)
	}
	close(events)
	return result
}

func TestDeleteWorktrees_DeletesRealDirectories(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	worktrees := []git.Worktree{
		{Path: dirA, Branch: "refs/heads/a"},
		{Path: dirB, Branch: "refs/heads/b"},
	}

	ch := make(chan progress.Event, 20)
	result := deleteWorktreesForTest(t, os.RemoveAll, worktrees, 5, ch)
	events := collectEvents(ch)

	if result.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d, want 2", result.SuccessCount)
	}
	if result.FailureCount != 0 {
		t.Errorf("FailureCount = %d, want 0", result.FailureCount)
	}
	if _, err := os.Stat(dirA); !os.IsNotExist(err) {
		t.Errorf("directory %s should be deleted", dirA)
	}
	if _, err := os.Stat(dirB); !os.IsNotExist(err) {
		t.Errorf("directory %s should be deleted", dirB)
	}

	var started, completed int
	for _, e := range events {
		switch e.Status {
		case progress.StepRunning:
			started++
		case progress.StepDone:
			completed++
		case progress.StepFailed:
			t.Error("unexpected failure event")
		}
	}
	if started != 4 {
		t.Errorf("started events = %d, want 4 (two checkpoints per worktree)", started)
	}
	if completed != 2 {
		t.Errorf("completed events = %d, want 2", completed)
	}
}

func TestDeleteWorktrees_DirectoryAlreadyMissing_Succeeds(t *testing.T) {
	worktrees := []git.Worktree{
		{Path: "/nonexistent/path/worktree"},
	}

	ch := make(chan progress.Event, 10)
	result := deleteWorktreesForTest(t, os.RemoveAll, worktrees, 5, ch)
	collectEvents(ch)

	if result.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1 (missing directory should be a no-op success)", result.SuccessCount)
	}
	if result.FailureCount != 0 {
		t.Errorf("FailureCount = %d, want 0", result.FailureCount)
	}
}

func TestDeleteWorktrees_RemovalFailure_ReportsFailure(t *testing.T) {
	removalErr := fmt.Errorf("permission denied")
	failingRemover := func(path string) error {
		return removalErr
	}

	worktrees := []git.Worktree{
		{Path: "/work/a"},
		{Path: "/work/b"},
	}

	ch := make(chan progress.Event, 20)
	result := deleteWorktreesForTest(t, failingRemover, worktrees, 5, ch)
	collectEvents(ch)

	if result.SuccessCount != 0 {
		t.Errorf("SuccessCount = %d, want 0", result.SuccessCount)
	}
	if result.FailureCount != 2 {
		t.Errorf("FailureCount = %d, want 2", result.FailureCount)
	}
	for i, o := range result.Outcomes {
		if o.Success {
			t.Errorf("Outcome[%d] should be failure", i)
		}
		if o.Error == nil {
			t.Errorf("Outcome[%d] should have error", i)
		}
	}
}

func TestDeleteWorktrees_MixedOutcomes(t *testing.T) {
	goodDir := t.TempDir()
	badPath := "/work/fail"

	removalErr := fmt.Errorf("locked")
	remover := func(path string) error {
		if path == badPath {
			return removalErr
		}
		return os.RemoveAll(path)
	}

	worktrees := []git.Worktree{
		{Path: goodDir},
		{Path: badPath},
	}

	ch := make(chan progress.Event, 20)
	result := deleteWorktreesForTest(t, remover, worktrees, 5, ch)
	events := collectEvents(ch)

	if result.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", result.SuccessCount)
	}
	if result.FailureCount != 1 {
		t.Errorf("FailureCount = %d, want 1", result.FailureCount)
	}
	if !result.Outcomes[0].Success {
		t.Error("first outcome should be success")
	}
	if result.Outcomes[1].Success {
		t.Error("second outcome should be failure")
	}

	var failed int
	for _, e := range events {
		if e.Status == progress.StepFailed {
			failed++
		}
	}
	if failed != 1 {
		t.Errorf("failed events = %d, want 1", failed)
	}
}

func TestDeleteWorktrees_EmptyInput(t *testing.T) {
	ch := make(chan progress.Event, 20)
	result := deleteWorktreesForTest(t, os.RemoveAll, nil, 5, ch)
	events := collectEvents(ch)

	if result.SuccessCount != 0 || result.FailureCount != 0 {
		t.Errorf("expected zero counts, got success=%d failure=%d", result.SuccessCount, result.FailureCount)
	}
	if len(events) != 0 {
		t.Errorf("expected no events, got %d", len(events))
	}
}

func TestDeleteWorktrees_ConcurrencyBound(t *testing.T) {
	const maxConcurrency = 2

	var current, maxConcurrent atomic.Int32
	// arrived signals that a worker has entered the remover.
	arrived := make(chan struct{}, 10)
	// gate holds workers until released.
	gate := make(chan struct{})

	remover := func(path string) error {
		cur := current.Add(1)
		defer current.Add(-1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		arrived <- struct{}{} // Signal arrival.
		<-gate                // Wait for release.
		return nil
	}

	worktrees := make([]git.Worktree, 5)
	for i := range worktrees {
		worktrees[i] = git.Worktree{Path: fmt.Sprintf("/work/%d", i+1)}
	}

	ch := make(chan progress.Event, 50)
	go func() {
		deleteWorktreesForTest(t, remover, worktrees, maxConcurrency, ch)
	}()

	// Wait for maxConcurrency workers to arrive (proves they're running concurrently).
	for range maxConcurrency {
		<-arrived
	}

	// Release all workers.
	close(gate)

	collectEvents(ch)

	if maxConcurrent.Load() > int32(maxConcurrency) {
		t.Errorf("max concurrent = %d, want <= %d", maxConcurrent.Load(), maxConcurrency)
	}
}

func TestPruneWorktrees_Success(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/repo:[worktree prune]": {Output: ""},
		},
	}

	err := PruneWorktrees(runner, "/repo")
	if err != nil {
		t.Errorf("PruneWorktrees() error = %v, want nil", err)
	}
}

func TestPruneWorktrees_Failure(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/repo:[worktree prune]": {Err: fmt.Errorf("prune failed")},
		},
	}

	err := PruneWorktrees(runner, "/repo")
	if err == nil {
		t.Error("PruneWorktrees() error = nil, want error")
	}
}

func TestUnlockWorktree_Success(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/repo:[worktree unlock /repo/wt]": {Output: ""},
		},
	}

	err := UnlockWorktree(runner, "/repo", "/repo/wt")
	if err != nil {
		t.Errorf("UnlockWorktree() error = %v, want nil", err)
	}
}

func TestUnlockWorktree_NotLocked_NoError(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/repo:[worktree unlock /repo/wt]": {Err: fmt.Errorf("git worktree unlock: fatal: '/repo/wt' is not locked")},
		},
	}

	err := UnlockWorktree(runner, "/repo", "/repo/wt")
	if err != nil {
		t.Errorf("UnlockWorktree() error = %v, want nil for already-unlocked worktree", err)
	}
}

func TestUnlockWorktree_UnlocksLockedWorktree(t *testing.T) {
	tmp := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = testtmp.HermeticGitEnv()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	repoPath := filepath.Join(tmp, "repo")
	run(tmp, "init", "--bare", "--initial-branch=main", repoPath)

	seed := filepath.Join(tmp, "_seed")
	run(tmp, "clone", repoPath, seed)
	run(seed, "config", "user.email", "test@test.com")
	run(seed, "config", "user.name", "Test")
	run(seed, "commit", "--allow-empty", "-m", "init")
	run(seed, "push", "origin", "main")

	wtPath := filepath.Join(tmp, "locked-wt")
	run(repoPath, "worktree", "add", "-b", "test-locked", wtPath)
	run(repoPath, "worktree", "lock", wtPath)

	// Verify it's locked — check for "locked" as a standalone line in porcelain output
	lockCheck := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	lockCheck.Env = testtmp.HermeticGitEnv()
	out, _ := lockCheck.CombinedOutput()
	if !containsLockedLine(string(out)) {
		t.Fatal("worktree should be locked")
	}

	runner := &git.GitRunner{}
	err := UnlockWorktree(runner, repoPath, wtPath)
	if err != nil {
		t.Fatalf("UnlockWorktree: %v", err)
	}

	// Verify it's no longer locked
	lockCheck2 := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	lockCheck2.Env = testtmp.HermeticGitEnv()
	out, _ = lockCheck2.CombinedOutput()
	if containsLockedLine(string(out)) {
		t.Fatal("worktree should no longer be locked after UnlockWorktree")
	}
}

// containsLockedLine reports whether s contains "locked" as a standalone line,
// as produced by git worktree list --porcelain. This avoids false positives from
// worktree paths that include the word "locked" (e.g. "locked-wt").
func containsLockedLine(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "locked" {
			return true
		}
	}
	return false
}

func TestUnlockWorktree_AlreadyUnlocked_NoError(t *testing.T) {
	tmp := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = testtmp.HermeticGitEnv()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	repoPath := filepath.Join(tmp, "repo")
	run(tmp, "init", "--bare", "--initial-branch=main", repoPath)

	seed := filepath.Join(tmp, "_seed")
	run(tmp, "clone", repoPath, seed)
	run(seed, "config", "user.email", "test@test.com")
	run(seed, "config", "user.name", "Test")
	run(seed, "commit", "--allow-empty", "-m", "init")
	run(seed, "push", "origin", "main")

	wtPath := filepath.Join(tmp, "unlocked-wt")
	run(repoPath, "worktree", "add", "-b", "test-unlocked", wtPath)

	runner := &git.GitRunner{}
	err := UnlockWorktree(runner, repoPath, wtPath)
	// Should not error — unlocking an already-unlocked worktree is a no-op
	if err != nil {
		t.Fatalf("UnlockWorktree on unlocked worktree: %v", err)
	}
}
