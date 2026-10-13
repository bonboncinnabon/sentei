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
// Protected branches, bare worktrees, and the worktree holding the current
// directory are always excluded: removing the last would leave the caller's
// shell in a directory that no longer exists.
// Locked worktrees are included so the caller can unlock them before deletion.
func ResolveFilters(worktrees []git.Worktree, opts *RemoveOptions, defaultBranch string, isMerged MergedChecker) []git.Worktree {
	now := time.Now()
	// An unreadable working directory leaves cwd empty, which matches nothing.
	cwd, _ := os.Getwd()
	var result []git.Worktree

	for _, wt := range worktrees {
		if wt.IsBare || git.PathContains(wt.Path, cwd) {
			continue
		}

		branch := shortBranch(wt.Branch)
		if git.IsProtectedBranchWith(wt.Branch, defaultBranch) {
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

// CheckMerged creates a MergedChecker that reports whether the default branch
// has absorbed a branch: every commit of the branch is in the default branch,
// and the default branch has moved past it. A branch still at the default
// branch's tip is a new branch with no commits of its own, not a merged one; a
// branch fast-forwarded into the default looks the same until the default gets
// another commit. The default branch is never merged into itself, so --merged
// can never select the default worktree.
func CheckMerged(runner git.CommandRunner, repoPath string, defaultBranch string) MergedChecker {
	return func(branch string) bool {
		if strings.EqualFold(branch, defaultBranch) {
			return false
		}
		// Full ref names: a bare name resolves to a tag of that name first.
		out, err := runner.Run(repoPath, "rev-list", "--left-right", "--count",
			"refs/heads/"+branch+"...refs/heads/"+defaultBranch)
		if err != nil {
			return false
		}
		var onlyInBranch, onlyInDefault int
		if _, err := fmt.Sscan(out, &onlyInBranch, &onlyInDefault); err != nil {
			return false
		}
		return onlyInBranch == 0 && onlyInDefault > 0
	}
}
