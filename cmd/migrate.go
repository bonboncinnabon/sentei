package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// RunMigrate executes the migrate command in non-interactive mode.
func RunMigrate(args []string) error {
	opts, err := ParseMigrateFlags(args)
	if err != nil {
		return err
	}
	if err := ValidateMigrateForNonInteractive(opts); err != nil {
		return err
	}

	repoPath := opts.RepoPath
	if absPath, err := filepath.Abs(repoPath); err == nil {
		repoPath = absPath
	}

	runner := &git.GitRunner{}

	context := repo.DetectContext(runner, repoPath)
	if context == repo.ContextBareRepo {
		return fmt.Errorf("repository is already bare: %s", repoPath)
	}
	if context != repo.ContextNonBareRepo {
		if err := repo.Unreachable(runner, repoPath); err != nil {
			return err
		}
		return fmt.Errorf("not a git repository: %s", repoPath)
	}

	result := repo.Migrate(runner, repo.MigrateOptions{RepoPath: repoPath}, printMigrateEvent)
	if err := result.Failure(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Printf("\n%s✓%s Migration complete\n", green, nc)
	fmt.Printf("  Bare root:  %s\n", result.BareRoot)
	fmt.Printf("  Worktree:   %s\n", result.WorktreePath)
	return nil
}

// printMigrateEvent prints the steps as create does, by their labels. Skipped
// steps are left out: after a refusal that is every step but the first.
func printMigrateEvent(e progress.Event) {
	if e.Status != progress.StepSkipped {
		printCreateEvent(e)
	}
}
