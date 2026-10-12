package creator

import (
	"errors"
	"fmt"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/worktreefile"
)

type Options struct {
	BranchName     string
	BaseBranch     string
	RepoPath       string
	SourceWorktree string
	MergeBase      bool
	CopyEnvFiles   bool
	// EnvFiles are copied from SourceWorktree when CopyEnvFiles is set, in
	// addition to the env files of Ecosystems. Callers pass the env files of
	// every ecosystem detected in the source, so copying them does not depend
	// on which installs were selected.
	EnvFiles      []string
	WorktreeFiles []worktreefile.Rule
	Ecosystems    []config.EcosystemConfig
}

type Result struct {
	// WorktreePath is set once the worktree exists on disk, even when a later
	// step failed. Empty means nothing was created.
	WorktreePath string
	// BranchExisted reports that the branch was already there and was checked
	// out as it is, so the base branch did not decide where it starts.
	BranchExisted bool
	Phases        []progress.Phase
	Err           error
}

func (r *Result) HasFailures() bool {
	return r.Err != nil || progress.PhasesHaveFailures(r.Phases)
}

func Run(runner git.CommandRunner, shell git.ShellRunner, opts Options, emit func(progress.Event)) Result {
	result := Result{}
	prepared, err := prepareCreation(opts)
	if err != nil {
		result.Err = err
		return result
	}
	execution, err := progress.Start(prepared.plan, emit)
	if err != nil {
		result.Err = fmt.Errorf("starting worktree creation: %w", err)
		return result
	}
	runErr := prepared.run(execution, runner, shell, &result)
	finishErr := execution.Finish("worktree creation finished")
	result.Phases = execution.Phases()
	result.Err = errors.Join(runErr, finishErr)
	return result
}
