package cleanup

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

type Mode string

const (
	ModeSafe       Mode = "safe"
	ModeAggressive Mode = "aggressive"
)

type Options struct {
	Mode   Mode
	Force  bool
	DryRun bool
}

// Result is what one cleanup run did. After a dry run every count is what a
// real run would do, and nothing was changed.
type Result struct {
	ConfigOrphanResult  ConfigResult
	ConfigDedupResult   ConfigResult // duplicate config lines
	StaleRefsRemoved    int
	GoneBranchesDeleted int
	// GoneBranches names the GoneBranchesDeleted branches.
	GoneBranches []string
	// NonWtBranchesDeleted names the branches aggressive mode deleted.
	NonWtBranchesDeleted   []string
	NonWtBranchesRemaining int
	WorktreesPruned        int
	BranchesSkipped        []SkippedBranch
	Errors                 []OperationError
}

// countPrunable counts worktree entries marked as "prunable" in porcelain output.
func countPrunable(porcelain string) int {
	count := 0
	for _, line := range strings.Split(porcelain, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "prunable") {
			count++
		}
	}
	return count
}

type ConfigResult struct {
	Removed int
}

type OperationError struct {
	Step string
	Err  error
}

type SkipReason string

const (
	SkipUnmerged   SkipReason = "not fully merged"
	SkipInWorktree SkipReason = "checked out in worktree"
	SkipProtected  SkipReason = "protected branch"
)

type SkippedBranch struct {
	Name   string
	Reason SkipReason
}

type Event struct {
	Step    string
	Message string
	Level   EventLevel
}

type EventLevel int

const (
	LevelStep EventLevel = iota
	LevelInfo
	LevelWarn
	LevelDetail
)

// Run executes every cleanup step in order. A failing step is recorded in
// Result.Errors and the remaining steps still run; whatever a step did before
// it failed stays in the counts.
func Run(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) Result {
	if err := git.ValidateRepository(runner, repoPath); err != nil {
		return Result{Errors: []OperationError{{Step: "repository", Err: err}}}
	}

	var result Result
	record := func(step string, err error) {
		if err != nil {
			result.Errors = append(result.Errors, OperationError{Step: step, Err: err})
		}
	}

	stale, err := PruneRemoteRefs(runner, repoPath, opts, emit)
	record("prune-refs", err)
	result.StaleRefsRemoved = len(stale)

	gone, err := DeleteGoneBranches(runner, repoPath, stale, opts, emit)
	record("gone-branches", err)
	result.GoneBranches = gone.Deleted
	result.GoneBranchesDeleted = len(gone.Deleted)
	result.BranchesSkipped = append(result.BranchesSkipped, gone.Skipped...)

	nonWorktree, err := CleanNonWorktreeBranches(runner, repoPath, stale, gone.Deleted, opts, emit)
	record("non-wt-branches", err)
	result.NonWtBranchesDeleted = nonWorktree.Deleted
	result.NonWtBranchesRemaining = nonWorktree.Remaining
	// An unmerged branch with a gone upstream is refused by both steps;
	// report it once.
	for _, skipped := range nonWorktree.Skipped {
		if !slices.Contains(result.BranchesSkipped, skipped) {
			result.BranchesSkipped = append(result.BranchesSkipped, skipped)
		}
	}

	result.ConfigOrphanResult, err = PurgeOrphanedBranchConfigs(runner, repoPath, opts, emit)
	record("orphaned-configs", err)

	result.ConfigDedupResult, err = DedupConfig(runner, repoPath, slices.Concat(gone.Deleted, nonWorktree.Deleted), opts, emit)
	record("dedup-config", err)

	result.WorktreesPruned, err = pruneWorktrees(runner, repoPath, opts, emit)
	record("worktree-prune", err)

	return result
}

// pruneWorktrees removes the metadata of worktrees whose directory is gone
// and returns how many there were. A dry run only counts them.
func pruneWorktrees(runner git.CommandRunner, repoPath string, opts Options, emit func(Event)) (int, error) {
	emit(Event{Level: LevelStep, Message: "Pruning stale worktree metadata"})

	output, err := runner.Run(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return 0, err
	}
	count := countPrunable(output)

	switch {
	case count == 0:
		emit(Event{Level: LevelInfo, Message: "No stale worktrees"})
	case opts.DryRun:
		emit(Event{Level: LevelDetail, Message: fmt.Sprintf("Would prune %d stale worktree(s)", count)})
	default:
		if _, err := runner.Run(repoPath, "worktree", "prune"); err != nil {
			return 0, err
		}
		emit(Event{Level: LevelInfo, Message: fmt.Sprintf("Pruned %d stale worktree(s)", count)})
	}
	return count, nil
}
