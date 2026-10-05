package cmd

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bonboncinnabon/sentei/internal/cli"
)

// RemoveOptions holds parsed flags for the remove command.
type RemoveOptions struct {
	Stale    time.Duration
	Merged   bool
	All      bool
	DryRun   bool
	Force    bool
	RepoPath string
}

// ParseStaleDuration parses human-friendly duration strings like "30d", "2w", "3m".
func ParseStaleDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q: must be a positive number followed by d, w, or m", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid duration %q: must be a positive number followed by d, w, or m", s)
	}

	var days int
	switch unit {
	case 'd':
		days = n
	case 'w':
		days = n * 7
	case 'm':
		days = n * 30
	default:
		return 0, fmt.Errorf("invalid duration %q: unit must be d (days), w (weeks), or m (months)", s)
	}

	return time.Duration(days) * 24 * time.Hour, nil
}

// ParseRemoveFlags parses remove-specific flags and returns RemoveOptions.
func ParseRemoveFlags(args []string) (*RemoveOptions, error) {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	stale := fs.String("stale", "", "Remove worktrees older than duration (e.g., 30d, 2w, 3m)")
	merged := fs.Bool("merged", false, "Remove worktrees whose branches are fully merged")
	all := fs.Bool("all", false, "Remove all non-protected worktrees")
	dryRun := fs.Bool("dry-run", false, "Show what would be removed without deleting")
	force := fs.Bool("force", false, "Remove at-risk worktrees (uncommitted, untracked, or unpushed work)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	opts := &RemoveOptions{
		Merged: *merged,
		All:    *all,
		DryRun: *dryRun,
		Force:  *force,
	}

	if *stale != "" {
		d, err := ParseStaleDuration(*stale)
		if err != nil {
			return nil, err
		}
		opts.Stale = d
	}

	if fs.NArg() > 0 {
		opts.RepoPath = fs.Arg(0)
	}

	return opts, nil
}

// ValidateRemoveForNonInteractive checks that at least one filter is specified
// for non-interactive execution.
func ValidateRemoveForNonInteractive(opts *RemoveOptions) error {
	if !opts.Merged && !opts.All && opts.Stale == 0 {
		return fmt.Errorf("at least one filter required: --stale, --merged, or --all")
	}
	return nil
}

// RemoveCLICommand generates the equivalent CLI command string from options.
func RemoveCLICommand(opts *RemoveOptions) string {
	flags := make(map[string]string)
	if opts.Stale > 0 {
		days := int(opts.Stale / (24 * time.Hour))
		flags["stale"] = fmt.Sprintf("%dd", days)
	}
	if opts.Merged {
		flags["merged"] = "true"
	}
	if opts.All {
		flags["all"] = "true"
	}
	if opts.DryRun {
		flags["dry-run"] = "true"
	}
	cmd := cli.BuildFlagString("sentei remove", flags)
	if opts.RepoPath != "" {
		cmd += " " + opts.RepoPath
	}
	return cmd
}

// FormatStaleDuration converts a duration back to a human-friendly string (in days).
func FormatStaleDuration(d time.Duration) string {
	days := int(d / (24 * time.Hour))
	return fmt.Sprintf("%dd", days)
}

// FormatFilterLabel generates a human-readable label for the active filters.
func FormatFilterLabel(opts *RemoveOptions) string {
	var parts []string
	if opts.All {
		return "all"
	}
	if opts.Merged {
		parts = append(parts, "merged")
	}
	if opts.Stale > 0 {
		parts = append(parts, "stale > "+FormatStaleDuration(opts.Stale))
	}
	return strings.Join(parts, ", ")
}
