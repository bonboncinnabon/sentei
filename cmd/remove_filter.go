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

// reflogStart is a date older than any reflog entry. Asked for a branch at
// that date, git answers with the commit the branch pointed at before its
// reflog began, or with the first logged commit when the reflog begins at the
// branch's creation.
const reflogStart = "1980-01-01"

// CheckMerged creates a MergedChecker that reports whether the default branch
// has absorbed a branch. The default branch must contain the branch's tip, and
// one of these must hold:
//
//   - the tip is not on the default branch's first-parent chain, so it arrived
//     through a merge commit;
//   - the tip is on that chain and the branch's reflog shows that it once
//     pointed at another commit: commits were made on the branch and the
//     default branch was fast-forwarded to them.
//
// A branch on the chain that never moved is one nobody has worked on, however
// far the default branch has moved since. A branch with no reflog (it is
// switched off, or its entries expired) cannot show that it moved and is not
// merged either. The default branch is never merged into itself, so --merged
// can never select the default worktree.
//
// The repository is read when the checker is created: one for-each-ref and one
// first-parent walk for the whole run, however many branches there are. A
// query then costs at most one reflog lookup.
func CheckMerged(runner git.CommandRunner, repoPath string, defaultBranch string) MergedChecker {
	never := func(string) bool { return false }
	// Full ref names: a bare name resolves to a tag of that name first.
	defaultRef := "refs/heads/" + defaultBranch

	contained, err := runner.Run(repoPath, "for-each-ref", "--merged="+defaultRef,
		"--format=%(objectname) %(refname:lstrip=2)", "refs/heads/")
	if err != nil {
		return never
	}
	tips := make(map[string]string)
	for line := range strings.SplitSeq(contained, "\n") {
		if tip, branch, ok := strings.Cut(line, " "); ok {
			tips[branch] = tip
		}
	}

	firstParents, err := runner.Run(repoPath, "rev-list", "--first-parent", defaultRef)
	if err != nil {
		return never
	}

	return func(branch string) bool {
		tip, ok := tips[branch]
		if !ok || strings.EqualFold(branch, defaultBranch) {
			return false
		}
		// A full object name cannot match across two lines of the list.
		if !strings.Contains(firstParents, tip) {
			return true
		}
		start, err := runner.Run(repoPath, "rev-parse", "--verify", "--quiet",
			"refs/heads/"+branch+"@{"+reflogStart+"}")
		return err == nil && start != tip
	}
}
