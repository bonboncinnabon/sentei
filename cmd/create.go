package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/ecosystem"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// RunCreate executes the create worktree command in non-interactive mode.
func RunCreate(args []string) error {
	opts, err := ParseCreateFlags(args)
	if err != nil {
		return err
	}
	if err := ValidateCreateForNonInteractive(opts); err != nil {
		return err
	}

	repoPath := "."
	if opts.RepoPath != "" {
		repoPath = opts.RepoPath
	}
	if absPath, err := filepath.Abs(repoPath); err == nil {
		repoPath = absPath
	}

	runner := &git.GitRunner{}
	shell := &git.DefaultShellRunner{}

	context := repo.DetectContext(runner, repoPath)
	if context != repo.ContextBareRepo {
		return fmt.Errorf("create requires a bare repository (detected: %v)", context)
	}

	// Normalize to the bare root: run from inside a worktree, the repo path is
	// that worktree, and the new one would be created nested inside it.
	repoPath = repo.ResolveBareRoot(runner, repoPath)

	// Build creator.Options from CLI flags.
	creatorOpts := creator.Options{
		BranchName:   opts.Branch,
		BaseBranch:   opts.Base,
		RepoPath:     repoPath,
		MergeBase:    opts.MergeBase,
		CopyEnvFiles: opts.CopyEnv,
	}

	cfg, err := config.LoadConfig(repoPath, config.WithRunner(runner))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
	}
	if cfg != nil {
		creatorOpts.WorktreeFiles = cfg.WorktreeFiles
		if len(opts.Ecosystems) > 0 {
			creatorOpts.Ecosystems = matchEcosystems(cfg.Ecosystems, opts.Ecosystems)
		}
	}

	// Find a source worktree for env file copying. The files to copy come from
	// the ecosystems detected there, not from --ecosystems: that flag selects
	// installs, and --copy-env must work without it.
	if opts.CopyEnv {
		worktrees, err := git.ListWorktrees(runner, repoPath)
		if err == nil {
			creatorOpts.SourceWorktree = creator.SourceWorktree(worktrees)
		}
		if cfg != nil && creatorOpts.SourceWorktree != "" {
			detected, _ := ecosystem.NewRegistry(cfg.Ecosystems).Detect(creatorOpts.SourceWorktree)
			for _, eco := range detected {
				creatorOpts.EnvFiles = append(creatorOpts.EnvFiles, eco.Config.EnvFiles...)
			}
		}
	}

	fmt.Printf("Creating worktree %q from %s...\n", opts.Branch, opts.Base)

	result := creator.Run(runner, shell, creatorOpts, func(e progress.Event) {
		printCreateEvent(e)
	})

	if err := createResultError(result); err != nil {
		return err
	}

	fmt.Printf("\n%sWorktree created:%s %s\n", green, nc, result.WorktreePath)
	return nil
}

// createResultError maps the run to the command's exit status and says
// whether a worktree was left behind, because that decides the next step.
func createResultError(result creator.Result) error {
	switch {
	case result.Err != nil:
		return fmt.Errorf("create failed: %w", result.Err)
	case result.WorktreePath == "":
		return errors.New("create failed: no worktree was created; fix the error above and run create again")
	case result.HasFailures():
		return fmt.Errorf("create completed with errors: the worktree exists at %s; resolve the failed steps above inside it", result.WorktreePath)
	}
	return nil
}

// printCreateEvent prints labels, not the Phase/Step identities: those are
// opaque hashes ("create-worktree:217d2bf51f16e9dc") meant for matching events.
func printCreateEvent(e progress.Event) {
	switch e.Status {
	case progress.StepRunning:
		fmt.Printf("%s→%s %s: %s\n", blue, nc, e.PhaseLabel, e.StepLabel)
	case progress.StepDone:
		msg := ""
		if e.Message != "" {
			msg = " — " + e.Message
		}
		fmt.Printf("%s✓%s %s%s\n", green, nc, e.StepLabel, msg)
	case progress.StepFailed:
		msg := ""
		if e.Error != nil {
			msg = " — " + e.Error.Error()
		}
		fmt.Printf("%s✗%s %s%s\n", yellow, nc, e.StepLabel, msg)
	case progress.StepSkipped:
		fmt.Printf("  %s%s (skipped)%s\n", dim, e.StepLabel, nc)
	}
}

// matchEcosystems filters configured ecosystems by the names requested via CLI.
func matchEcosystems(available []config.EcosystemConfig, requested []string) []config.EcosystemConfig {
	want := make(map[string]bool, len(requested))
	for _, name := range requested {
		want[name] = true
	}
	var matched []config.EcosystemConfig
	for _, eco := range available {
		if want[eco.Name] {
			matched = append(matched, eco)
		}
	}
	return matched
}
