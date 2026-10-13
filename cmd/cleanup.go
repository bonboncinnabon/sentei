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
	printCleanupBranches(result, opts.DryRun)
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

// printCleanupBranches names the branches the run deleted and the ones it
// left alone, one per line with the reason. The step lines above only count
// them. After a dry run it names what the real run would do.
func printCleanupBranches(result cleanup.Result, dryRun bool) {
	deleted, skipped, deletedStyle := "Deleted", "Skipped", green
	if dryRun {
		deleted, skipped, deletedStyle = "Would delete", "Would skip", ""
	}

	if count := len(result.GoneBranchesDeleted) + len(result.NonWtBranchesDeleted); count > 0 {
		fmt.Printf("%s%s %d branch(es):%s\n", deletedStyle, deleted, count, nc)
		for _, name := range result.GoneBranchesDeleted {
			fmt.Printf("  %s  (upstream gone)\n", name)
		}
		for _, name := range result.NonWtBranchesDeleted {
			fmt.Printf("  %s  (not in any worktree)\n", name)
		}
		fmt.Println()
	}
	if len(result.BranchesSkipped) > 0 {
		fmt.Printf("%s%s %d branch(es):%s\n", yellow, skipped, len(result.BranchesSkipped), nc)
		for _, branch := range result.BranchesSkipped {
			fmt.Printf("  %s  (%s)\n", branch.Name, branch.Reason)
		}
		fmt.Println()
	}
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
