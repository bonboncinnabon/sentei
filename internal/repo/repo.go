package repo

import (
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

type RepoContext int

const (
	ContextBareRepo    RepoContext = iota // bare repo with worktrees — full menu
	ContextNonBareRepo                    // regular git repo — offer migrate
	ContextNoRepo                         // not in a git repo — offer create/clone
)

// String names the context in user-facing messages.
func (c RepoContext) String() string {
	switch c {
	case ContextBareRepo:
		return "bare repository"
	case ContextNonBareRepo:
		return "regular repository"
	default:
		return "no repository"
	}
}

// DetectContext reports what kind of repository holds path. A bare layout is
// any repository whose shared git directory is a bare repository, whatever
// that directory is called and wherever its worktrees live; path may be the
// bare directory, the container of a .bare layout, or any worktree. Git is
// asked at the shared directory because inside a worktree it answers for the
// worktree, which is never bare.
func DetectContext(runner git.CommandRunner, path string) RepoContext {
	commonDir, err := git.CommonDir(runner, path)
	if err != nil {
		return ContextNoRepo
	}
	if bare, err := runner.Run(commonDir, "rev-parse", "--is-bare-repository"); err == nil && bare == "true" {
		return ContextBareRepo
	}
	return ContextNonBareRepo
}

// ResolveBareRoot resolves the root that holds a repository's worktrees from
// any path into it. See git.RepoRoot for the rule.
func ResolveBareRoot(runner git.CommandRunner, path string) string {
	return git.RepoRoot(runner, path)
}

func (r CloneResult) HasFailures() bool { return r.Err != nil || progress.PhasesHaveFailures(r.Phases) }
func (r CreateResult) HasFailures() bool {
	return r.Err != nil || progress.PhasesHaveFailures(r.Phases)
}
func (r MigrateResult) HasFailures() bool {
	return r.Err != nil || progress.PhasesHaveFailures(r.Phases)
}
