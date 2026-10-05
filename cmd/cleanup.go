package cmd

import (
	"fmt"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
)

const (
	green  = "\033[0;32m"
	yellow = "\033[1;33m"
	blue   = "\033[0;34m"
	dim    = "\033[2m"
	nc     = "\033[0m"
)

// RunCleanup executes the cleanup command in non-interactive mode.
func RunCleanup(args []string) error {
	opts, err := ParseCleanupFlags(args)
	if err != nil {
		return err
	}
	if err := ValidateCleanupForNonInteractive(opts); err != nil {
		return err
	}
	repoPath := ParseCleanupRepoPath(args)
	return RunCleanupWithOpts(opts, repoPath)
}

// RunCleanupWithOpts executes cleanup with pre-parsed options.
func RunCleanupWithOpts(opts *cleanup.Options, repoPath string) error {
	if opts.DryRun {
		fmt.Printf("%s(dry run)%s\n", dim, nc)
	}
	fmt.Println()

	runner := &git.GitRunner{}
	result := cleanup.Run(runner, repoPath, *opts, printEvent)

	fmt.Println()
	for _, e := range result.Errors {
		fmt.Printf("%s⚠%s  %s: %s\n", yellow, nc, e.Step, e.Err)
	}

	if result.NonWtBranchesRemaining > 0 && opts.Mode == cleanup.ModeSafe {
		fmt.Printf("\n%sTip:%s %d local branch(es) are not checked out in any worktree.\n", blue, nc, result.NonWtBranchesRemaining)
		fmt.Printf("     Run %ssentei cleanup --mode=aggressive%s to remove them.\n", dim, nc)
	}

	return nil
}

func printEvent(e cleanup.Event) {
	switch e.Level {
	case cleanup.LevelStep:
		fmt.Printf("%s→%s %s\n", blue, nc, e.Message)
	case cleanup.LevelInfo:
		fmt.Printf("%s✓%s %s\n", green, nc, e.Message)
	case cleanup.LevelWarn:
		fmt.Printf("%s⚠%s  %s\n", yellow, nc, e.Message)
	case cleanup.LevelDetail:
		fmt.Printf("  %s%s%s\n", dim, e.Message, nc)
	}
}
