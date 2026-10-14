package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
// bare directory, the container of a .bare layout, or any worktree.
//
// Git answers for the shared directory because --git-dir names it; asked
// plainly inside a worktree it answers for the worktree, which is never bare.
// The directory is named and not entered for two reasons: git refuses a bare
// directory it has to discover when safe.bareRepository is "explicit", and
// inside a .git directory whose config has no core.bare it guesses "bare".
func DetectContext(runner git.CommandRunner, path string) RepoContext {
	commonDir, err := git.CommonDir(runner, path)
	if err != nil {
		return ContextNoRepo
	}
	if bare, err := runner.Run(path, "--git-dir="+commonDir, "rev-parse", "--is-bare-repository"); err == nil && bare == "true" {
		return ContextBareRepo
	}
	return ContextNonBareRepo
}

// Unreachable explains why git finds no repository at path when path holds a
// bare layout that git will not enter, and how to fix that; otherwise it
// returns nil. sentei does not enter such a repository itself. A .bare
// directory with no .git file beside it, and a bare repository under
// safe.bareRepository=explicit, are what git's discovery passes over on
// purpose: a bare repository planted in a downloaded tree runs its config's
// commands for whoever enters it.
func Unreachable(runner git.CommandRunner, path string) error {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if info, err := os.Stat(filepath.Join(path, ".bare")); err == nil && info.IsDir() {
		pointer := filepath.Join(path, ".git")
		if _, err := os.Lstat(pointer); errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s holds .bare but no .git file pointing at it, so git finds no repository there; if .bare is your repository, recreate the file with: printf 'gitdir: ./.bare\\n' > '%s'", path, pointer)
		}
	}
	if _, err := runner.Run(path, "rev-parse", "--git-dir"); err != nil && strings.Contains(err.Error(), "safe.bareRepository") {
		return fmt.Errorf("git does not enter the bare repository at %s because safe.bareRepository is explicit; run sentei from one of its worktrees", path)
	}
	return nil
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
