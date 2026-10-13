package repo

import (
	"path/filepath"

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

// DetectContext determines the repo context at the given path.
//
// Detection logic:
//  1. git rev-parse --is-bare-repository → "true" means ContextBareRepo
//  2. Check for .bare directory at repo root (sentei's bare structure from a worktree)
//  3. git rev-parse --git-dir succeeds → ContextNonBareRepo
//  4. Otherwise → ContextNoRepo
func DetectContext(runner git.CommandRunner, path string) RepoContext {
	output, err := runner.Run(path, "rev-parse", "--is-bare-repository")
	if err == nil && output == "true" {
		return ContextBareRepo
	}

	// Check if git repo at all
	_, err = runner.Run(path, "rev-parse", "--git-dir")
	if err != nil {
		return ContextNoRepo
	}

	// Inside a git repo — sentei's bare layout has a ".bare" common dir.
	// (--show-toplevel returns the worktree root, not the bare repo root.)
	if commonDir, err := git.CommonDir(runner, path); err == nil && filepath.Base(commonDir) == ".bare" {
		return ContextBareRepo
	}

	return ContextNonBareRepo
}

// ResolveBareRoot resolves the root that holds a repository's worktrees from
// any path into it: a worktree, the container of a .bare layout, or the bare
// directory. For a .bare layout that is the container, also when path is the
// .bare directory itself. A plain bare repository is its own root. Falls back
// to path when git cannot resolve the common dir.
func ResolveBareRoot(runner git.CommandRunner, path string) string {
	commonDir, err := git.CommonDir(runner, path)
	if err != nil || (commonDir == path && filepath.Base(commonDir) != ".bare") {
		return path
	}
	return filepath.Dir(commonDir)
}

func (r CloneResult) HasFailures() bool { return r.Err != nil || progress.PhasesHaveFailures(r.Phases) }
func (r CreateResult) HasFailures() bool {
	return r.Err != nil || progress.PhasesHaveFailures(r.Phases)
}
func (r MigrateResult) HasFailures() bool {
	return r.Err != nil || progress.PhasesHaveFailures(r.Phases)
}
