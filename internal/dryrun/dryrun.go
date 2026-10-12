package dryrun

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

// Print writes the worktree summary to w. defaultBranch is the repository's
// detected default branch: its worktree is marked [P] like the built-in
// protected names, because removal protects it the same way.
func Print(worktrees []git.Worktree, defaultBranch string, w io.Writer) error {
	sorted := make([]git.Worktree, len(worktrees))
	copy(sorted, worktrees)
	sort.SliceStable(sorted, func(a, b int) bool {
		aZero := sorted[a].LastCommitDate.IsZero()
		bZero := sorted[b].LastCommitDate.IsZero()
		if aZero != bZero {
			return !aZero
		}
		if aZero && bZero {
			return false
		}
		return sorted[a].LastCommitDate.Before(sorted[b].LastCommitDate)
	})

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "STATUS\tBRANCH\tAGE\tSUBJECT"); err != nil {
		return err
	}
	for _, wt := range sorted {
		branch := worktree.Label(wt)

		age := worktree.RelativeTime(wt.LastCommitDate)
		subject := wt.LastCommitSubject
		if wt.EnrichmentError != "" {
			age = "error"
			subject = wt.EnrichmentError
		}

		if git.IsProtectedBranchWith(wt.Branch, defaultBranch) {
			branch += " [P]"
		}

		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", worktree.StatusBadge(wt), branch, age, subject); err != nil {
			return err
		}
	}
	return tw.Flush()
}
