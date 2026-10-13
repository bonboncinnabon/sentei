package worktree

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// The plain-text vocabulary every surface uses for a worktree: the TUI list,
// the confirm screen, the progress rows and the --dry-run table. One
// definition keeps a worktree recognisable from one to the next.

// ShortBranch strips refs/heads/ from a branch ref.
func ShortBranch(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

// Label is the display name of a worktree: its branch, the short HEAD hash
// for a detached HEAD, "(prunable)" for one whose directory is gone, or the
// directory name as a last resort.
func Label(wt git.Worktree) string {
	if branch := ShortBranch(wt.Branch); branch != "" {
		return branch
	}
	switch {
	case wt.IsDetached && len(wt.HEAD) >= 7:
		return wt.HEAD[:7]
	case wt.IsPrunable:
		return "(prunable)"
	}
	return filepath.Base(wt.Path)
}

// StatusBadge is the status column for a worktree. The first match wins:
// locked, unreadable, uncommitted changes, untracked files, clean.
func StatusBadge(wt git.Worktree) string {
	switch {
	case wt.IsLocked:
		return "[L]"
	case wt.EnrichmentError != "":
		// Unknown is not clean: the row must not read as [ok].
		return "[?]"
	case wt.HasUncommittedChanges:
		return "[~]"
	case wt.HasUntrackedFiles:
		return "[!]"
	default:
		return "[ok]"
	}
}

// RelativeTime says how long ago t was in the largest whole unit ("3 days
// ago"), or "unknown" for the zero time.
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	count, unit := int(d.Hours()/24/365), "year"
	switch {
	case d < time.Hour:
		count, unit = int(d.Minutes()), "minute"
	case d < 24*time.Hour:
		count, unit = int(d.Hours()), "hour"
	case d < 30*24*time.Hour:
		count, unit = int(d.Hours()/24), "day"
	case d < 365*24*time.Hour:
		count, unit = int(d.Hours()/24/30), "month"
	}
	if count == 1 {
		return "1 " + unit + " ago"
	}
	return fmt.Sprintf("%d %ss ago", count, unit)
}
