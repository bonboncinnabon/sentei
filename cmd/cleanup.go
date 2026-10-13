package cmd

import (
	"fmt"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// RunCleanup executes the cleanup command in non-interactive mode.
func RunCleanup(args []string) error {
	opts, repoPath, err := ParseCleanupFlags(args)
	if err != nil {
		return err
	}
	if err := ValidateCleanupForNonInteractive(opts); err != nil {
		return err
	}
	return RunCleanupWithOpts(opts, repoPath)
}

// RunCleanupWithOpts executes cleanup with pre-parsed options.
func RunCleanupWithOpts(opts *cleanup.Options, repoPath string) error {
	if opts.DryRun {
		fmt.Printf("%s(dry run)%s\n", dim, nc)
	}
	fmt.Println()

	runner := &git.GitRunner{}
	// Cleanup acts on the whole repository, so it runs from the repository
	// root like the TUI does. Inside a worktree git judges "merged" against
	// that worktree's branch instead of the default branch.
	if repo.DetectContext(runner, repoPath) == repo.ContextBareRepo {
		repoPath = repo.ResolveBareRoot(runner, repoPath)
	}
	result := cleanup.Run(runner, repoPath, *opts, printEvent)

	fmt.Println()
	for _, e := range result.Errors {
		fmt.Printf("%s⚠%s  %s: %s\n", yellow, nc, e.Step, e.Err)
	}

	if result.NonWtBranchesRemaining > 0 && opts.Mode == cleanup.ModeSafe {
		fmt.Printf("\n%sTip:%s %d local branch(es) are not checked out in any worktree.\n", blue, nc, result.NonWtBranchesRemaining)
		fmt.Printf("     Run %ssentei cleanup --mode=aggressive%s to remove them.\n", dim, nc)
	}

	// A script or agent reads the exit status, not the warnings above.
	if len(result.Errors) > 0 {
		return fmt.Errorf("cleanup finished with %d failed step(s)", len(result.Errors))
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
