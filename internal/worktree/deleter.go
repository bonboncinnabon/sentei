package worktree

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

// RemovalPhaseName is the canonical phase under which worktree deletions
// report progress; the TUI renders the same phase name.
const RemovalPhaseName = "Removing worktrees"

const RemovalPhaseID progress.PhaseID = "remove-worktrees"

type RemovalTarget struct {
	Worktree git.Worktree
	StepID   progress.StepID
}

type WorktreeOutcome struct {
	Path    string
	Success bool
	Error   error
}

type DeletionResult struct {
	SuccessCount int
	FailureCount int
	Outcomes     []WorktreeOutcome
	Phases       []progress.Phase
	Err          error
}

func (r DeletionResult) HasFailures() bool {
	return r.Err != nil || r.FailureCount > 0 || progress.PhasesHaveFailures(r.Phases)
}

func PruneWorktrees(runner git.CommandRunner, repoPath string) error {
	_, err := runner.Run(repoPath, "worktree", "prune")
	return err
}

func UnlockWorktree(runner git.CommandRunner, repoPath, wtPath string) error {
	_, err := runner.Run(repoPath, "worktree", "unlock", wtPath)
	if err != nil && strings.Contains(err.Error(), "is not locked") {
		return nil // already unlocked — not an error
	}
	return err
}

func DeleteWorktrees(execution *progress.Execution, phaseID progress.PhaseID, remover func(string) error, targets []RemovalTarget, maxConcurrency int) DeletionResult {
	if len(targets) == 0 {
		return DeletionResult{}
	}

	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}

	var mu sync.Mutex
	result := DeletionResult{
		Outcomes: make([]WorktreeOutcome, len(targets)),
	}
	recordExecutionError := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		result.Err = errors.Join(result.Err, err)
		mu.Unlock()
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for i, target := range targets {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, target RemovalTarget) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := execution.Running(phaseID, target.StepID, 1, "Removing from disk"); err != nil {
				recordExecutionError(err)
				return
			}

			err := remover(target.Worktree.Path)

			mu.Lock()
			if err != nil {
				result.FailureCount++
				result.Outcomes[idx] = WorktreeOutcome{
					Path:    target.Worktree.Path,
					Success: false,
					Error:   fmt.Errorf("removing %s: %w", target.Worktree.Path, err),
				}
			} else {
				result.SuccessCount++
				result.Outcomes[idx] = WorktreeOutcome{
					Path:    target.Worktree.Path,
					Success: true,
				}
			}
			mu.Unlock()

			if err != nil {
				_, progressErr := execution.Fail(phaseID, target.StepID, err)
				recordExecutionError(progressErr)
				return
			}
			if progressErr := execution.Running(phaseID, target.StepID, 2, "Removed from disk"); progressErr != nil {
				recordExecutionError(progressErr)
				return
			}
			_, progressErr := execution.Done(phaseID, target.StepID, "Removed")
			recordExecutionError(progressErr)
		}(i, target)
	}

	wg.Wait()
	return result
}
