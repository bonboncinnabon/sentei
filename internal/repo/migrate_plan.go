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

	// Submodules is the number of submodules checked out in the working
	// tree, which the conversion carries over.
	Submodules int

	submodules []submodule
	// state is what git reported for the checkout before the conversion. The
	// new worktree must report the same, or the conversion is rolled back.
	state []string
}

// submodule is a submodule whose working directory points at a repository
// under .git/modules. Everything the rollback puts back is read up front.
type submodule struct {
	work   string      // working directory, relative to the repository root
	gitDir string      // its repository
	link   []byte      // content of the .git file in the working directory
	mode   fs.FileMode // and its permissions
	config []byte      // content of the repository's config
	// absorb marks a submodule the superproject tracks: git moves its
	// repository into the new worktree's own modules directory.
	absorb bool
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
	plan := MigratePlan{RepoPath: root}
	if included := includedFromWorkingTree(runner, root); included != "" {
		return MigratePlan{}, fmt.Errorf("the repository config includes %s, which moves with the working tree, and git skips an include it cannot find; remove the include from .git/config, migrate, then add it back with the new path", included)
	}

	plan.Branch, err = runner.Run(root, "branch", "--show-current")
	if err != nil {
		return MigratePlan{}, fmt.Errorf("reading the current branch: %w", err)
	}
	if plan.Branch == "" {
		return MigratePlan{}, errors.New("HEAD is detached; check out a branch first (git switch <branch>), then retry")
	}
	if _, err := runner.Run(root, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return MigratePlan{}, fmt.Errorf("branch %q has no commits yet; make a first commit, then retry", plan.Branch)
	}
	plan.WorktreePath = git.WorktreePath(root, plan.Branch)
	plan.DefaultBranch = migrateDefaultBranch(runner, root, plan.Branch)

	if exists(plan.WorktreePath) {
		return MigratePlan{}, fmt.Errorf("%s already exists and is where the %q worktree goes; rename it or switch to another branch, then retry", plan.WorktreePath, plan.Branch)
	}
	if plan.submodules, err = submodulesUnder(runner, root, filepath.Join(gitDir, "modules"), false); err != nil {
		return MigratePlan{}, err
	}
	if plan.state, err = checkoutState(runner, root, nil); err != nil {
		return MigratePlan{}, fmt.Errorf("reading the working tree status: %w", err)
	}

	plan.Submodules = len(plan.submodules)
	return plan, nil
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

// submodulesUnder finds the repositories under a modules directory whose
// working directory points back at them, parents before the submodules
// nested in them. A repository without such a working directory (a submodule
// that was deinitialised, or belongs to another branch) is left out: it moves
// with the modules directory and git links it again when it is checked out.
func submodulesUnder(runner git.CommandRunner, root, modules string, nested bool) ([]submodule, error) {
	entries, err := os.ReadDir(modules)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading the submodule repositories: %w", err)
	}
	var found []submodule
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(modules, entry.Name())
		below, belowNested := dir, nested
		if head, err := os.Lstat(filepath.Join(dir, "HEAD")); err == nil && head.Mode().IsRegular() {
			sub, active, err := readSubmodule(runner, root, dir)
			if err != nil {
				return nil, err
			}
			if !active {
				continue
			}
			// git absorbs a nested repository together with its parent.
			sub.absorb = sub.absorb && !nested
			found = append(found, sub)
			below, belowNested = filepath.Join(dir, "modules"), true
		}
		// Without a HEAD the directory is part of a submodule name that has
		// a slash in it, and the repositories are further down.
		inner, err := submodulesUnder(runner, root, below, belowNested)
		if err != nil {
			return nil, err
		}
		found = append(found, inner...)
	}
	return found, nil
}

// readSubmodule describes the submodule whose repository is gitDir. It is
// active when the working directory the repository names has a .git file
// that points back.
func readSubmodule(runner git.CommandRunner, root, gitDir string) (sub submodule, active bool, err error) {
	configPath := filepath.Join(gitDir, "config")
	work, err := runner.Run(root, "config", "--file", configPath, "--get", "core.worktree")
	if err != nil {
		return submodule{}, false, nil
	}
	if !filepath.IsAbs(work) {
		work = filepath.Join(gitDir, work)
	}
	gitfile := filepath.Join(work, ".git")
	link, target, err := readGitfile(gitfile)
	if err != nil || !sameFile(target, gitDir) {
		return submodule{}, false, nil
	}
	inside, ok := pathInside(root, work)
	if !ok {
		return submodule{}, false, fmt.Errorf("the submodule repository %s has its working directory at %s, outside the repository; remove that submodule (git submodule deinit), then retry", gitDir, work)
	}
	info, err := os.Lstat(gitfile)
	if err != nil {
		return submodule{}, false, fmt.Errorf("reading %s: %w", gitfile, err)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		return submodule{}, false, fmt.Errorf("reading %s: %w", configPath, err)
	}
	// A submodule the index does not list (left behind by a branch switch)
	// is not git's to absorb: it keeps its repository inside it.
	entry, _ := runner.Run(root, "ls-files", "--stage", "--", ":(literal)"+filepath.ToSlash(inside))
	return submodule{
		work: inside, gitDir: gitDir, link: link, mode: info.Mode().Perm(), config: config,
		absorb: strings.HasPrefix(entry, "160000 "),
	}, true, nil
}

// checkoutProbes are the questions whose answers must be the same for the new
// worktree as they were for the checkout. An optional one fails when there is
// nothing to ask about (a gitlink that .gitmodules does not name stops the
// submodule listing), and is only asked again when it had an answer.
var checkoutProbes = []struct {
	name     string
	args     []string
	optional bool
}{
	// Without the optional index lock, so a check never rewrites the index.
	{name: "git status", args: []string{"--no-optional-locks", "status", "--porcelain"}},
	{name: "git submodule status", args: []string{"submodule", "status", "--recursive"}, optional: true},
}

// checkoutState answers checkoutProbes for the checkout at path. With before,
// the answers of an earlier call, an optional probe that had none is skipped.
func checkoutState(runner git.CommandRunner, path string, before []string) ([]string, error) {
	state := make([]string, len(checkoutProbes))
	for index, probe := range checkoutProbes {
		if probe.optional && before != nil && before[index] == "" {
			continue
		}
		answer, err := runner.Run(path, probe.args...)
		if err != nil && !probe.optional {
			return nil, err
		}
		state[index] = answer
	}
	return state, nil
}

// readGitfile reads a .git file and returns its content and the directory it
// points at. A relative path counts from the directory that holds the file.
func readGitfile(path string) (content []byte, target string, err error) {
	content, err = os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	pointer, ok := strings.CutPrefix(strings.TrimRight(string(content), "\r\n"), "gitdir: ")
	if !ok {
		return nil, "", fmt.Errorf("%s is not a .git file", path)
	}
	if !filepath.IsAbs(pointer) {
		pointer = filepath.Join(filepath.Dir(path), pointer)
	}
	return content, filepath.Clean(pointer), nil
}

// pathInside returns dir relative to root when it lies inside root. Symlinks
// are resolved first: git records resolved paths, and the root may have been
// reached through a symlink.
func pathInside(root, dir string) (string, bool) {
	root, rootErr := filepath.EvalSymlinks(root)
	dir, dirErr := filepath.EvalSymlinks(dir)
	inside, err := filepath.Rel(root, dir)
	return inside, rootErr == nil && dirErr == nil && err == nil && inside != "." && filepath.IsLocal(inside)
}

// sameFile reports whether two paths lead to the same file or directory.
func sameFile(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
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
