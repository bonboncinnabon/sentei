package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// MergedChecker checks whether a branch is fully merged into the default branch.
type MergedChecker func(branch string) bool

// ResolveFilters returns the worktrees that match the given filter options.
// Filters combine with OR logic: a worktree matching any active filter is included.
// Protected branches (built-in + custom) and bare worktrees are always excluded.
// Locked worktrees are included so the caller can unlock them before deletion.
func ResolveFilters(worktrees []git.Worktree, opts *RemoveOptions, protectedBranches []string, defaultBranch string, isMerged MergedChecker) []git.Worktree {
	protectedSet := make(map[string]bool, len(protectedBranches))
	for _, b := range protectedBranches {
		protectedSet[b] = true
	}

	now := time.Now()
	var result []git.Worktree

	for _, wt := range worktrees {
		if wt.IsBare {
			continue
		}

		branch := shortBranch(wt.Branch)
		if git.IsProtectedBranchWith(wt.Branch, defaultBranch) || protectedSet[branch] {
			continue
		}

		if matchesFilters(wt, opts, now, isMerged, branch) {
			result = append(result, wt)
		}
	}

	return result
}

func matchesFilters(wt git.Worktree, opts *RemoveOptions, now time.Time, isMerged MergedChecker, branch string) bool {
	if opts.All {
		return true
	}

	if opts.Stale > 0 {
		if wt.LastCommitDate.IsZero() {
			fmt.Fprintf(os.Stderr, "Warning: skipping worktree %s (no commit date available)\n", wt.Path)
		} else if now.Sub(wt.LastCommitDate) > opts.Stale {
			return true
		}
	}

	if opts.Merged && isMerged != nil && branch != "" {
		if isMerged(branch) {
			return true
		}
	}

	return false
}

func shortBranch(branch string) string {
	return strings.TrimPrefix(branch, "refs/heads/")
}

// CheckMerged creates a MergedChecker that uses git merge-base --is-ancestor
// to determine if a branch is fully merged into the default branch. The default
// branch is never reported as merged into itself (a branch is its own ancestor),
// so --merged can never select the default worktree.
func CheckMerged(runner git.CommandRunner, repoPath string, defaultBranch string) MergedChecker {
	return func(branch string) bool {
		if strings.EqualFold(branch, defaultBranch) {
			return false
		}
		_, err := runner.Run(repoPath, "merge-base", "--is-ancestor", branch, defaultBranch)
		return err == nil
	}
}
