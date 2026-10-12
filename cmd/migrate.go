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

func printMigrateEvent(e progress.Event) {
	switch e.Status {
	case progress.StepRunning:
		msg := ""
		if e.Message != "" {
			msg = fmt.Sprintf(" — %s", e.Message)
		}
		fmt.Printf("%s→%s [%s] %s%s\n", blue, nc, e.Phase, e.Step, msg)
	case progress.StepDone:
		msg := ""
		if e.Message != "" {
			msg = fmt.Sprintf(" (%s)", e.Message)
		}
		fmt.Printf("%s✓%s [%s] %s%s\n", green, nc, e.Phase, e.Step, msg)
	case progress.StepFailed:
		fmt.Printf("%s✗%s [%s] %s: %v\n", yellow, nc, e.Phase, e.Step, e.Error)
	}
}
