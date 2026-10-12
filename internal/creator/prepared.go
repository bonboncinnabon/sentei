package creator

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktreefile"
)

const (
	setupPhaseID        progress.PhaseID = "setup"
	dependenciesPhaseID progress.PhaseID = "dependencies"
)

type preparedDependency struct {
	stepID  progress.StepID
	label   string
	command string
}

type preparedCreation struct {
	opts               Options
	plan               progress.Plan
	worktreePath       string
	createStepID       progress.StepID
	mergeStepID        progress.StepID
	worktreeFileStepID progress.StepID
	envStepID          progress.StepID
	envFiles           []string
	dependencies       []preparedDependency
	hasDependencies    bool
}

func prepareCreation(opts Options) (preparedCreation, error) {
	if strings.TrimSpace(opts.BranchName) == "" || strings.TrimSpace(opts.BaseBranch) == "" || strings.TrimSpace(opts.RepoPath) == "" {
		return preparedCreation{}, errors.New("preparing worktree creation: branch, base branch, and repository path are required")
	}
	// git worktree add hands both names to an internal `git branch` call with
	// no "--" of its own, so "-D" as a branch name would delete the base.
	for _, name := range []string{opts.BranchName, opts.BaseBranch} {
		if strings.HasPrefix(name, "-") {
			return preparedCreation{}, fmt.Errorf("preparing worktree creation: branch name %q starts with \"-\" and git would read it as an option; use a name that starts with a letter or digit", name)
		}
	}
	if err := validateEcosystemIdentities(opts); err != nil {
		return preparedCreation{}, fmt.Errorf("preparing worktree creation: %w", err)
	}
	if err := worktreefile.ValidateRules(opts.WorktreeFiles); err != nil {
		return preparedCreation{}, fmt.Errorf("preparing worktree creation: %w", err)
	}

	prepared := preparedCreation{
		opts: opts, worktreePath: git.WorktreePath(opts.RepoPath, opts.BranchName),
		createStepID: semanticStepID("create-worktree", opts.BranchName),
	}
	setup := progress.PlannedPhase{ID: setupPhaseID, Label: "Setup"}
	setup.Steps = append(setup.Steps, progress.PlannedStep{ID: prepared.createStepID, Label: "Create worktree"})
	if opts.MergeBase {
		prepared.mergeStepID = semanticStepID("merge-base", opts.BaseBranch)
		setup.Steps = append(setup.Steps, progress.PlannedStep{ID: prepared.mergeStepID, Label: "Merge base branch"})
	}
	if len(opts.WorktreeFiles) > 0 {
		prepared.worktreeFileStepID = semanticStepID("copy-worktree-files", worktreeFileIdentity(opts.WorktreeFiles))
		setup.Steps = append(setup.Steps, progress.PlannedStep{ID: prepared.worktreeFileStepID, Label: "Copy worktree files"})
	}
	if opts.CopyEnvFiles {
		prepared.envFiles = uniqueEnvFiles(opts)
		if len(prepared.envFiles) > 0 {
			prepared.envStepID = semanticStepID("copy-env", strings.Join(prepared.envFiles, "\x00"))
			setup.Steps = append(setup.Steps, progress.PlannedStep{ID: prepared.envStepID, Label: "Copy env files"})
		}
	}
	prepared.plan.Phases = append(prepared.plan.Phases, setup)

	// One install per ecosystem, run at the worktree root. Workspace-aware
	// package managers install every member from there and share one store,
	// so a per-member install would only repeat the work.
	dependencyPhase := progress.PlannedPhase{ID: dependenciesPhaseID, Label: "Dependencies"}
	for _, ecosystem := range opts.Ecosystems {
		command := strings.TrimSpace(ecosystem.Install.Command)
		if command == "" {
			continue
		}
		operation := preparedDependency{
			stepID: semanticStepID("install-dependencies", ecosystem.Name), label: ecosystem.Name, command: command,
		}
		prepared.dependencies = append(prepared.dependencies, operation)
		dependencyPhase.Steps = append(dependencyPhase.Steps, progress.PlannedStep{ID: operation.stepID, Label: operation.label})
	}
	if len(dependencyPhase.Steps) > 0 {
		prepared.hasDependencies = true
		prepared.plan.Phases = append(prepared.plan.Phases, dependencyPhase)
	}
	return prepared, nil
}

func validateEcosystemIdentities(opts Options) error {
	seen := make(map[string]bool, len(opts.Ecosystems))
	for _, ecosystem := range opts.Ecosystems {
		name := strings.TrimSpace(ecosystem.Name)
		if name == "" {
			return errors.New("ecosystem has empty name")
		}
		if seen[name] {
			return fmt.Errorf("duplicate ecosystem identity %q", name)
		}
		seen[name] = true
	}
	return nil
}

func (p preparedCreation) run(execution *progress.Execution, runner git.CommandRunner, shell git.ShellRunner, result *Result) error {
	_, err := execution.Run(setupPhaseID, p.createStepID, func() (string, error) {
		return p.addWorktree(runner, result)
	})
	if err != nil {
		return fmt.Errorf("executing worktree creation: %w", err)
	}
	if result.WorktreePath == "" {
		return p.skipBlocked(execution, "blocked by Create worktree")
	}

	if p.mergeStepID != "" {
		_, err := execution.Run(setupPhaseID, p.mergeStepID, func() (string, error) {
			_, err := runner.Run(p.worktreePath, "merge", "--no-edit", "--", p.opts.BaseBranch)
			return "", err
		})
		if err != nil {
			return fmt.Errorf("executing merge: %w", err)
		}
	}
	if p.worktreeFileStepID != "" {
		if err := p.runWorktreeFileCopy(execution); err != nil {
			return err
		}
	}
	if p.envStepID != "" {
		_, err := execution.Run(setupPhaseID, p.envStepID, func() (string, error) {
			return copyPreparedEnvFiles(p.opts.SourceWorktree, p.worktreePath, p.envFiles)
		})
		if err != nil {
			return fmt.Errorf("executing env copy: %w", err)
		}
	}
	var runErr error
	if p.hasDependencies {
		runErr = errors.Join(runErr, p.runDependencies(execution, shell))
	}
	return runErr
}

// addWorktree runs git worktree add and records on result what now exists on
// disk. The base only starts a branch that exists nowhere: a local branch is
// checked out as it is, and one that only origin has becomes a local branch
// tracking it. The remote ref is named in full because git refuses to guess
// when a second remote carries the same branch. Refs follow "--", so a name
// beginning with "-" is never read as an option.
func (p preparedCreation) addWorktree(runner git.CommandRunner, result *Result) (string, error) {
	branch, base := p.opts.BranchName, p.opts.BaseBranch
	remoteRef := "refs/remotes/origin/" + branch
	args := []string{"worktree", "add", "-b", branch, "--", p.worktreePath, base}
	existing, message := "", p.worktreePath
	if git.BranchExists(runner, p.opts.RepoPath, branch) {
		args = []string{"worktree", "add", "--", p.worktreePath, branch}
		existing = branch
		message = fmt.Sprintf("%s (checked out existing branch %s; base %s was not applied)", p.worktreePath, branch, base)
	} else if _, err := runner.Run(p.opts.RepoPath, "show-ref", "--verify", remoteRef); err == nil {
		args = []string{"worktree", "add", "--track", "-b", branch, "--", p.worktreePath, remoteRef}
		existing = "origin/" + branch
		message = fmt.Sprintf("%s (checked out remote branch %s; base %s was not applied)", p.worktreePath, existing, base)
	}
	_, statErr := os.Lstat(p.worktreePath)
	pathWasFree := errors.Is(statErr, os.ErrNotExist)

	_, err := runner.Run(p.opts.RepoPath, args...)
	if err != nil {
		// git removes a worktree it could not check out, but keeps one whose
		// post-checkout hook failed and still exits non-zero. A .git entry in
		// a directory that was absent before the command is that worktree.
		if _, gitErr := os.Stat(filepath.Join(p.worktreePath, ".git")); !pathWasFree || gitErr != nil {
			return "", fmt.Errorf("creating worktree: %w", err)
		}
	}
	result.WorktreePath = p.worktreePath
	result.ExistingBranch = existing
	if err != nil {
		return "", fmt.Errorf("worktree created, but git reported an error (a post-checkout hook may have failed): %w", err)
	}
	return message, nil
}

func (p preparedCreation) skipBlocked(execution *progress.Execution, reason string) error {
	var err error
	err = errors.Join(err, execution.SkipPending(setupPhaseID, reason))
	if p.hasDependencies {
		err = errors.Join(err, execution.SkipPending(dependenciesPhaseID, reason))
	}
	return err
}

func (p preparedCreation) runDependencies(execution *progress.Execution, shell git.ShellRunner) error {
	var runErr error
	for _, dependency := range p.dependencies {
		runErr = errors.Join(runErr, runPreparedDependency(execution, shell, p.worktreePath, dependency))
	}
	return runErr
}

func runPreparedDependency(execution *progress.Execution, shell git.ShellRunner, worktreePath string, dependency preparedDependency) error {
	_, err := execution.Run(dependenciesPhaseID, dependency.stepID, func() (string, error) {
		_, err := shell.RunShell(worktreePath, dependency.command)
		if err != nil {
			return "", fmt.Errorf("installing %s: %w", dependency.label, err)
		}
		return "", nil
	})
	if err != nil {
		return fmt.Errorf("executing dependency %s: %w", dependency.label, err)
	}
	return nil
}

func semanticStepID(kind, identity string) progress.StepID {
	sum := sha256.Sum256([]byte(identity))
	return progress.StepID(fmt.Sprintf("%s:%x", kind, sum[:8]))
}
