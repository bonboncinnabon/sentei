package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// MigratePlan describes an in-place conversion that PlanMigrate has cleared.
type MigratePlan struct {
	RepoPath      string // absolute repository root; it becomes the container
	Branch        string // branch checked out in the working tree
	WorktreePath  string // directory the working tree moves into
	DefaultBranch string // branch the bare HEAD will name

	// status is `git status --porcelain` before the conversion. The new
	// worktree must report the same, or the conversion is rolled back.
	status string
}

// migrateBlockers are entries under .git that hold state the conversion
// cannot carry into a linked worktree. A directory only counts when it has
// content.
var migrateBlockers = []struct{ path, reason string }{
	{"index.lock", "another git command is running in this repository (.git/index.lock exists); wait for it to finish, then retry"},
	{"MERGE_HEAD", "a merge is in progress; finish or abort it (git merge --abort), then retry"},
	{"CHERRY_PICK_HEAD", "a cherry-pick is in progress; finish or abort it (git cherry-pick --abort), then retry"},
	{"REVERT_HEAD", "a revert is in progress; finish or abort it (git revert --abort), then retry"},
	{"sequencer", "a cherry-pick or revert sequence is in progress; finish or abort it (--continue or --abort), then retry"},
	{"rebase-merge", "a rebase is in progress; finish or abort it (git rebase --abort), then retry"},
	{"rebase-apply", "a rebase or git am is in progress; finish or abort it (git rebase --abort or git am --abort), then retry"},
	{"BISECT_LOG", "a bisect is in progress; end it (git bisect reset), then retry"},
	{"modules", "the repository has submodules checked out, which migrate cannot relocate; clone the remote with `sentei clone` instead"},
	{"worktrees", "the repository already has linked worktrees; remove them (git worktree remove <path>, or git worktree prune for deleted ones), migrate, then recreate them"},
}

// PlanMigrate checks that the repository at repoPath can be converted in
// place and reports what the conversion will do. It changes nothing.
func PlanMigrate(runner git.CommandRunner, repoPath string) (MigratePlan, error) {
	root, err := filepath.Abs(repoPath)
	if err != nil {
		return MigratePlan{}, fmt.Errorf("resolving repository path %q: %w", repoPath, err)
	}
	gitDir := filepath.Join(root, ".git")
	switch info, err := os.Lstat(gitDir); {
	case errors.Is(err, fs.ErrNotExist):
		if top, err := runner.Run(root, "rev-parse", "--show-toplevel"); err == nil && top != "" {
			return MigratePlan{}, fmt.Errorf("%s is inside a git repository, not at its root; migrate %s instead", root, top)
		}
		return MigratePlan{}, fmt.Errorf("%s is not a git repository (it has no .git directory); there is nothing to migrate", root)
	case err != nil:
		return MigratePlan{}, fmt.Errorf("reading %s: %w", gitDir, err)
	case !info.IsDir():
		if exists(filepath.Join(root, ".bare")) {
			return MigratePlan{}, fmt.Errorf("%s already uses the bare layout (its .git points at .bare); there is nothing to migrate", root)
		}
		return MigratePlan{}, fmt.Errorf("%s is a linked worktree or submodule (its .git is a file, not a directory); migrate the main checkout instead", root)
	}
	if exists(filepath.Join(root, ".bare")) {
		return MigratePlan{}, fmt.Errorf("%s already has a .bare entry, which the conversion needs for the repository data; move it away and retry", root)
	}
	for _, blocker := range migrateBlockers {
		if hasContent(filepath.Join(gitDir, blocker.path)) {
			return MigratePlan{}, errors.New(blocker.reason)
		}
	}
	for _, key := range []string{"extensions.worktreeConfig", "core.sparseCheckout"} {
		// A linked worktree reads these from its own admin directory, and
		// core.bare would have to move with them.
		if value, _ := runner.Run(root, "config", "--type=bool", "--get", key); value == "true" {
			return MigratePlan{}, fmt.Errorf("sparse checkout or per-worktree config is enabled (%s), which migrate cannot carry over; disable it, or clone the remote with `sentei clone` instead", key)
		}
	}
	if worktree, _ := runner.Run(root, "config", "--local", "--get", "core.worktree"); worktree != "" {
		return MigratePlan{}, fmt.Errorf("core.worktree is set (%s), which a bare repository cannot have; if the working tree is this directory, unset it (git config --unset core.worktree), then retry", worktree)
	}
	if included := includedFromWorkingTree(runner, root); included != "" {
		return MigratePlan{}, fmt.Errorf("the repository config includes %s, which moves with the working tree, and git skips an include it cannot find; remove the include from .git/config, migrate, then add it back with the new path", included)
	}

	branch, err := runner.Run(root, "branch", "--show-current")
	if err != nil {
		return MigratePlan{}, fmt.Errorf("reading the current branch: %w", err)
	}
	if branch == "" {
		return MigratePlan{}, errors.New("HEAD is detached; check out a branch first (git switch <branch>), then retry")
	}
	if _, err := runner.Run(root, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return MigratePlan{}, fmt.Errorf("branch %q has no commits yet; make a first commit, then retry", branch)
	}
	worktreePath := git.WorktreePath(root, branch)
	if exists(worktreePath) {
		return MigratePlan{}, fmt.Errorf("%s already exists and is where the %q worktree goes; rename it or switch to another branch, then retry", worktreePath, branch)
	}
	status, err := worktreeStatus(runner, root)
	if err != nil {
		return MigratePlan{}, fmt.Errorf("reading the working tree status: %w", err)
	}
	return MigratePlan{
		RepoPath: root, Branch: branch, WorktreePath: worktreePath,
		DefaultBranch: migrateDefaultBranch(runner, root, branch), status: status,
	}, nil
}

// includedFromWorkingTree returns a config file that .git/config includes
// from inside the working tree, or "" when there is none. An include that
// resolves into .git or outside the repository keeps resolving from .bare.
func includedFromWorkingTree(runner git.CommandRunner, root string) string {
	// With -z the output is origin, NUL, key, NUL, repeated, and paths are
	// not quoted.
	entries, _ := runner.Run(root, "config", "--local", "--includes", "--show-origin", "--name-only", "--list", "-z")
	fields := strings.Split(entries, "\x00")
	for index := 0; index+1 < len(fields); index += 2 {
		origin := strings.TrimPrefix(fields[index], "file:")
		if !filepath.IsAbs(origin) {
			origin = filepath.Join(root, origin)
		}
		if git.PathContains(root, origin) && !git.PathContains(filepath.Join(root, ".git"), origin) {
			return filepath.Clean(origin)
		}
	}
	return ""
}

// migrateDefaultBranch picks the branch the bare HEAD should name: the
// remote's default, then main, then master, whichever exists locally first.
// With none of them it stays on the branch that is checked out.
func migrateDefaultBranch(runner git.CommandRunner, root, current string) string {
	candidates := []string{"main", "master"}
	if remoteHead, err := runner.Run(root, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		candidates = append([]string{strings.TrimPrefix(remoteHead, "origin/")}, candidates...)
	}
	for _, candidate := range candidates {
		if git.BranchExists(runner, root, candidate) {
			return candidate
		}
	}
	return current
}

// worktreeStatus reads the status without taking the optional index lock, so
// the index file is never rewritten by a check.
func worktreeStatus(runner git.CommandRunner, path string) (string, error) {
	return runner.Run(path, "--no-optional-locks", "status", "--porcelain")
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

// hasContent reports whether path exists and, for a directory, is not empty.
func hasContent(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	entries, err := os.ReadDir(path)
	return err != nil || len(entries) > 0
}
