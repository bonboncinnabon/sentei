package tui

import (
	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
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

	pruneErr      *error
	cleanupResult *cleanup.Result
}

type unlockOperation struct {
	stepID   progress.StepID
	worktree git.Worktree
}

type removalPreparation struct {
	plan      progress.Plan
	unlockOps []unlockOperation
	targets   []worktree.RemovalTarget
}
