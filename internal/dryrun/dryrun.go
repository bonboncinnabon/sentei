package dryrun

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func Print(worktrees []git.Worktree, w io.Writer) error {
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
		branch := stripBranchPrefix(wt.Branch)
		if branch == "" {
			switch {
			case wt.IsDetached:
				branch = wt.HEAD
				if len(branch) >= 7 {
					branch = branch[:7]
				}
			case wt.IsPrunable:
				branch = "(prunable)"
			}
		}

		age := relativeTime(wt.LastCommitDate)
		subject := wt.LastCommitSubject
		if wt.EnrichmentError != "" {
			age = "error"
			subject = wt.EnrichmentError
		}

		if git.IsProtectedBranch(wt.Branch) {
			branch += " [P]"
		}

		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", statusIndicator(wt), branch, age, subject); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func statusIndicator(wt git.Worktree) string {
	switch {
	case wt.IsLocked:
		return "[L]"
	case wt.HasUncommittedChanges:
		return "[~]"
	case wt.HasUntrackedFiles:
		return "[!]"
	default:
		return "[ok]"
	}
}

func stripBranchPrefix(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
