package cleanup

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// BranchCleanResult is the outcome of one branch-deleting step.
type BranchCleanResult struct {
	// Deleted names the branches the step deleted, or would delete in a dry run.
	Deleted   []string
	Remaining int
	Skipped   []SkippedBranch
}

// DeleteGoneBranches deletes local branches whose upstream no longer exists.
// stale is what PruneRemoteRefs returned. A dry run leaves those refs in
// place, so git does not yet report their branches as gone and the list
// stands in for the prune.
//
// The returned result is valid alongside a non-nil error: it holds what was
// deleted before a deletion failed.
func DeleteGoneBranches(runner git.CommandRunner, repoPath string, stale []string, opts Options, emit func(Event)) (BranchCleanResult, error) {
	emit(Event{Step: "gone-branches", Message: "Deleting branches with gone upstream...", Level: LevelStep})

	// A real run has pruned by now, so git itself reports which upstreams are
	// gone. Only a dry run has to work from the list.
	if !opts.DryRun {
		stale = nil
	}

	gone, kept, err := listGoneBranches(runner, repoPath, stale)
	if err != nil {
		return BranchCleanResult{}, err
	}
	result := BranchCleanResult{Skipped: kept}

	if len(gone) == 0 {
		if len(kept) == 0 {
			emit(Event{Step: "gone-branches", Message: "No branches with gone upstream", Level: LevelInfo})
		}
		return result, nil
	}

	// Safe mode never force-deletes, with or without --force: discarding
	// unmerged work is what aggressive mode is for.
	force := opts.Force && opts.Mode == ModeAggressive

	deleted, unmerged, err := deleteBranches(runner, repoPath, gone, stale, force, opts.DryRun)
	result.Deleted = deleted
	result.Skipped = append(result.Skipped, unmerged...)

	if len(deleted) > 0 {
		emit(deletedEvent("gone-branches", opts.DryRun, len(deleted), "branch(es) with gone upstream"))
	}
	if len(unmerged) > 0 {
		emit(Event{Step: "gone-branches", Message: fmt.Sprintf("%d branch(es) skipped (not fully merged)", len(unmerged)), Level: LevelWarn})
	}

	return result, err
}

// CleanNonWorktreeBranches deletes, in aggressive mode, the local branches
// that are in no worktree and not protected. Safe mode only counts them.
// stale and goneDeleted are what the two earlier steps returned: a dry run
// did not apply them, so they are taken into account here instead.
//
// The returned result is valid alongside a non-nil error: it holds what was
// deleted before a deletion failed.
func CleanNonWorktreeBranches(runner git.CommandRunner, repoPath string, stale, goneDeleted []string, opts Options, emit func(Event)) (BranchCleanResult, error) {
	emit(Event{Step: "non-wt-branches", Message: "Checking non-worktree branches...", Level: LevelStep})

	candidates, err := listNonWorktreeCandidates(runner, repoPath, goneDeleted)
	if err != nil {
		return BranchCleanResult{}, err
	}

	var result BranchCleanResult

	if opts.Mode != ModeAggressive {
		result.Remaining = len(candidates)
		if len(candidates) > 0 {
			emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("%d branch(es) not in any worktree", len(candidates)), Level: LevelDetail})
		} else {
			emit(Event{Step: "non-wt-branches", Message: "No non-worktree branches", Level: LevelInfo})
		}
		return result, nil
	}

	result.Deleted, result.Skipped, err = deleteBranches(runner, repoPath, candidates, stale, opts.Force, opts.DryRun)
	result.Remaining = len(candidates) - len(result.Deleted)

	if len(result.Deleted) > 0 {
		emit(deletedEvent("non-wt-branches", opts.DryRun, len(result.Deleted), "non-worktree branch(es)"))
	}
	if len(result.Skipped) > 0 {
		emit(Event{Step: "non-wt-branches", Message: fmt.Sprintf("%d branch(es) skipped (not fully merged, use --force to delete them)", len(result.Skipped)), Level: LevelWarn})
	}

	return result, err
}

// deletedEvent words a deletion count for a real run or a dry run.
func deletedEvent(step string, dryRun bool, count int, what string) Event {
	if dryRun {
		return Event{Step: step, Message: fmt.Sprintf("Would delete %d %s", count, what), Level: LevelDetail}
	}
	return Event{Step: step, Message: fmt.Sprintf("Deleted %d %s", count, what), Level: LevelInfo}
}

// deleteBranches deletes each name with `git branch -d`, or `-D` when force
// is set, and keeps going after a failure. A branch git refuses because it is
// not fully merged is a skip; any other failure (a held ref lock, a branch
// checked out since the scan) is joined into the returned error.
//
// A dry run deletes nothing and returns what the real run would: the same
// split into deleted and unmerged, judged as if the stale refs were pruned.
func deleteBranches(runner git.CommandRunner, repoPath string, names, stale []string, force, dryRun bool) (deleted []string, unmerged []SkippedBranch, err error) {
	deleteFlag := "-d"
	if force {
		deleteFlag = "-D"
	}

	var failures []error
	for _, name := range names {
		if dryRun {
			if force || branchDeletableByGit(runner, repoPath, name, stale) {
				deleted = append(deleted, name)
			} else {
				unmerged = append(unmerged, SkippedBranch{Name: name, Reason: SkipUnmerged})
			}
			continue
		}

		_, deleteErr := runner.Run(repoPath, "branch", deleteFlag, name)
		switch {
		case deleteErr == nil:
			deleted = append(deleted, name)
		case !force && !branchDeletableByGit(runner, repoPath, name, nil):
			unmerged = append(unmerged, SkippedBranch{Name: name, Reason: SkipUnmerged})
		default:
			failures = append(failures, fmt.Errorf("deleting branch %s: %w", name, deleteErr))
		}
	}
	return deleted, unmerged, errors.Join(failures...)
}

// listNonWorktreeCandidates returns local branches that are neither checked
// out in any worktree nor protected — the set aggressive cleanup deletes.
// exclude names branches to leave out: the ones an earlier step deleted, or
// in a dry run would have.
func listNonWorktreeCandidates(runner git.CommandRunner, repoPath string, exclude []string) ([]string, error) {
	wtOutput, err := runner.Run(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}
	wtBranches := parseWorktreeBranches(wtOutput)

	branches, err := localBranches(runner, repoPath)
	if err != nil {
		return nil, err
	}

	// The repo's real default branch may be non-standard ("trunk"); it is not
	// checked out when no worktree holds it, so the worktree set alone would
	// let aggressive mode delete it.
	protected := git.DetectDefaultBranch(runner, repoPath)

	var candidates []string
	for _, b := range branches {
		if wtBranches[b] || git.IsProtectedBranchWith(b, protected) || slices.Contains(exclude, b) {
			continue
		}
		candidates = append(candidates, b)
	}
	return candidates, nil
}

// localBranches returns the names of the local branches. It asks
// for-each-ref, not `git branch`, which adds a "(no branch)" entry in a
// worktree with a detached HEAD. lstrip=2, not short: `short` abbreviates to
// "heads/<name>" when a tag shares the branch's name.
func localBranches(runner git.CommandRunner, repoPath string) ([]string, error) {
	output, err := runner.Run(repoPath, "for-each-ref", "--format=%(refname:lstrip=2)", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}
	return strings.Fields(output), nil
}

// goneBranchFormat asks for-each-ref for a branch's name, its upstream ref,
// the upstream's tracking state and the worktree that has it checked out,
// separated by a byte no ref name can contain. Unlike `git branch -vv` this
// output is not translated and holds no commit subject.
const goneBranchFormat = "--format=%(refname:lstrip=2)\x1f%(upstream)\x1f%(upstream:track)\x1f%(worktreepath)"

// listGoneBranches finds the local branches whose upstream no longer exists,
// or will not once the stale refs are pruned. gone holds the ones cleanup may
// delete; kept holds the ones it must leave alone, with the reason.
func listGoneBranches(runner git.CommandRunner, repoPath string, stale []string) (gone []string, kept []SkippedBranch, err error) {
	output, err := runner.Run(repoPath, "for-each-ref", goneBranchFormat, "refs/heads")
	if err != nil {
		return nil, nil, fmt.Errorf("listing branches: %w", err)
	}

	pendingPrune := make(map[string]bool, len(stale))
	for _, ref := range stale {
		pendingPrune["refs/remotes/"+ref] = true
	}
	protected := git.DetectDefaultBranch(runner, repoPath)

	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(line, "\x1f", 4)
		if len(fields) != 4 {
			continue
		}
		name, upstream, track, worktreePath := fields[0], fields[1], fields[2], fields[3]
		if track != "[gone]" && !pendingPrune[upstream] {
			continue
		}
		switch {
		case git.IsProtectedBranchWith(name, protected):
			kept = append(kept, SkippedBranch{Name: name, Reason: SkipProtected})
		case worktreePath != "":
			kept = append(kept, SkippedBranch{Name: name, Reason: SkipInWorktree})
		default:
			gone = append(gone, name)
		}
	}
	return gone, kept, nil
}

func parseWorktreeBranches(output string) map[string]bool {
	branches := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "branch refs/heads/") {
			b := strings.TrimPrefix(line, "branch refs/heads/")
			branches[b] = true
		}
	}
	return branches
}
