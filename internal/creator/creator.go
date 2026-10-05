package creator

import (
	"errors"
	"fmt"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/integration"
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
	WorktreeFiles  []worktreefile.Rule
	Ecosystems     []config.EcosystemConfig
	Integrations   []integration.Integration
}

type Result struct {
	WorktreePath string
	Phases       []progress.Phase
	Err          error
}

func (r *Result) HasFailures() bool {
	return r.Err != nil || progress.PhasesHaveFailures(r.Phases)
}

func Run(runner git.CommandRunner, shell git.ShellRunner, opts Options, emit func(progress.Event)) Result {
	result := Result{}
	prepared, err := prepareCreation(runner, shell, opts)
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
