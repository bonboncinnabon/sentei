package tui

import (
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

// removalRun holds all state for a single deletion run. It is created fresh
// when the user confirms a deletion, so no outcomes or results from a
// previous run can leak into the next one. targets is a snapshot of the
// selection at confirm time; the live list may be reloaded while the run
// renders.
type removalRun struct {
	result     worktree.DeletionResult
	progressCh chan progress.Event
	execution  *progress.Execution
	events     []progress.Event
	targets    []worktree.RemovalTarget

	// pruned is set once the closing `git worktree prune` has returned,
	// which ends the run's work. pruneErr is its error, if it had one.
	pruned   bool
	pruneErr error
}

type removalPreparation struct {
	plan    progress.Plan
	targets []worktree.RemovalTarget
}
