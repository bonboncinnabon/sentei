package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

type MigrateOptions struct {
	RepoPath string
}

type MigrateResult struct {
	BareRoot     string
	WorktreePath string
	Branch       string
	Phases       []progress.Phase
	Err          error
}

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

const (
	migrateValidatePhaseID progress.PhaseID = "migrate:validate"
	migrateConvertPhaseID  progress.PhaseID = "migrate:convert"
	migrateRollbackPhaseID progress.PhaseID = "migrate:rollback"
	migrateRollbackStepID  progress.StepID  = "restore"
)

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

// perWorktreeFiles live in .git for a normal checkout and in the worktree's
// own admin directory for a linked worktree (`git rev-parse --git-path <name>`
// says which). They follow the working tree: left behind in .bare, the next
// commit would lose the message a squash or cherry-pick prepared for it, and
// ORIG_HEAD and FETCH_HEAD would stop resolving.
var perWorktreeFiles = []string{
	"index", filepath.Join("logs", "HEAD"),
	"ORIG_HEAD", "FETCH_HEAD", "AUTO_MERGE",
	"MERGE_MSG", "SQUASH_MSG", "COMMIT_EDITMSG",
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

// Migrate converts a normal checkout into sentei's layout without copying or
// deleting anything: .git becomes .bare and the working tree, with its index,
// moves into a worktree directory named after the current branch. Every
// change is a rename or a small git edit with an undo, so a failure at any
// point puts the original layout back.
func Migrate(runner git.CommandRunner, opts MigrateOptions, emit func(progress.Event)) MigrateResult {
	migrate, finish := prepareMigrate(runner, opts)
	return finish(migrate.run(emit))
}

// prepareMigrate declares the migrate flow and returns the function that
// turns its outcome into a MigrateResult.
func prepareMigrate(runner git.CommandRunner, opts MigrateOptions) (flow, func([]progress.Phase, error) MigrateResult) {
	var plan MigratePlan
	// The paths below hang off the absolute root. PlanMigrate resolves the
	// same path and refuses first if that fails.
	root, _ := filepath.Abs(opts.RepoPath)
	gitDir := filepath.Join(root, ".git")
	barePath := filepath.Join(root, ".bare")
	configPath := filepath.Join(barePath, "config")
	var gitMoved, stateMoved, filesMoved renames
	var originalConfig []byte // the repository config before the first git edit
	registered := false       // git created the worktree directory and its .git file

	migrate := flow{name: "repository migration"}
	migrate.add(migrateValidatePhaseID, "Validate", step{id: "preflight", label: "Check repository", run: func() (string, error) {
		var err error
		plan, err = PlanMigrate(runner, opts.RepoPath)
		return plan.Branch, err
	}})
	migrate.add(migrateConvertPhaseID, "Convert", step{
		id: "move-git", label: "Move .git to .bare",
		run:  func() (string, error) { return "", gitMoved.move(gitDir, barePath) },
		undo: gitMoved.undo,
	})
	// The pointer comes before any git command: from here on git reaches .bare
	// through it. A bare repository git finds on its own is refused when
	// safe.bareRepository is "explicit".
	migrate.add(migrateConvertPhaseID, "Convert", step{
		id: "git-pointer", label: "Write .git pointer",
		run: func() (string, error) {
			return "", os.WriteFile(gitDir, []byte("gitdir: .bare\n"), 0o644)
		},
		undo: func() error { return removeFile(gitDir) },
	})
	migrate.add(migrateConvertPhaseID, "Convert", step{
		id: "mark-bare", label: "Mark repository as bare",
		run: func() (string, error) {
			var err error
			if originalConfig, err = os.ReadFile(configPath); err != nil {
				return "", err
			}
			_, err = runner.Run(root, "config", "core.bare", "true")
			return "", err
		},
		undo: func() error {
			if originalConfig == nil {
				return nil
			}
			// The file goes back as it was read. Unsetting core.bare alone
			// would not be enough: registering the worktree can add an
			// extension and raise the repository format version.
			if err := os.WriteFile(configPath, originalConfig, 0o644); err == nil {
				return nil
			}
			// A read-only config cannot be written in place. git replaces
			// the file instead, as it did for the edit being undone.
			_, err := runner.Run(root, "config", "core.bare", "false")
			return err
		},
	})
	migrate.add(migrateConvertPhaseID, "Convert", step{
		id: "worktree", label: "Register worktree",
		run: func() (string, error) {
			// --no-checkout registers the worktree without writing a single
			// file: the existing files and index are moved in instead.
			if _, err := runner.Run(root, "worktree", "add", "--no-checkout", plan.WorktreePath, plan.Branch); err != nil {
				return "", err
			}
			registered = true
			adminDir, err := runner.Run(plan.WorktreePath, "rev-parse", "--absolute-git-dir")
			if err != nil {
				return "", err
			}
			names := slices.Clone(perWorktreeFiles)
			// A split index keeps most of its entries in sharedindex.<hash>.
			// The names are matched one by one: a glob would read brackets
			// in the repository path as part of the pattern.
			entries, err := os.ReadDir(barePath)
			if err != nil {
				return "", err
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "sharedindex.") {
					names = append(names, entry.Name())
				}
			}
			for _, name := range names {
				from := filepath.Join(barePath, name)
				if !exists(from) {
					continue
				}
				// Not every git version creates logs/ for a fresh worktree.
				to := filepath.Join(adminDir, name)
				if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
					return "", err
				}
				if err := stateMoved.move(from, to); err != nil {
					return "", err
				}
			}
			return plan.WorktreePath, nil
		},
		undo: func() error {
			if !registered {
				// git cleans up after a failed add; nothing here is ours.
				return nil
			}
			// The index goes back first: pruning deletes the admin directory
			// it sits in. The directory is removed only when empty, so
			// nothing of the user's can be deleted here.
			if err := stateMoved.undo(); err != nil {
				return err
			}
			if err := removeFile(filepath.Join(plan.WorktreePath, ".git")); err != nil {
				return err
			}
			if err := os.Remove(plan.WorktreePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			_, err := runner.Run(root, "worktree", "prune")
			return err
		},
	})
	migrate.add(migrateConvertPhaseID, "Convert", step{
		id: "move-files", label: "Move working tree",
		run: func() (string, error) {
			entries, err := os.ReadDir(root)
			if err != nil {
				return "", err
			}
			for _, entry := range entries {
				name := entry.Name()
				if name == ".git" || name == ".bare" || name == filepath.Base(plan.WorktreePath) {
					continue
				}
				if err := filesMoved.move(filepath.Join(root, name), filepath.Join(plan.WorktreePath, name)); err != nil {
					return "", fmt.Errorf("moving %s into the worktree: %w", name, err)
				}
			}
			return fmt.Sprintf("%d entries", len(filesMoved)), nil
		},
		undo: filesMoved.undo,
	})
	// The checkout's HEAD reflog moved into the worktree. Git would start a
	// new one in .bare for these switches, which the rollback would then find
	// in the place the original has to return to.
	pointHEAD := func(branch string) error {
		_, err := runner.Run(root, "-c", "core.logAllRefUpdates=false", "symbolic-ref", "HEAD", "refs/heads/"+branch)
		return err
	}
	migrate.add(migrateConvertPhaseID, "Convert", step{
		id: "default-branch", label: "Point HEAD at the default branch",
		run: func() (string, error) {
			// Until now HEAD was the checkout's. In the bare repository it
			// names the default branch, which sentei protects and measures
			// "merged" against. The worktree keeps its own HEAD.
			return plan.DefaultBranch, pointHEAD(plan.DefaultBranch)
		},
		undo: func() error { return pointHEAD(plan.Branch) },
	})
	migrate.add(migrateConvertPhaseID, "Convert", step{id: "verify", label: "Verify working tree", run: func() (string, error) {
		status, err := worktreeStatus(runner, plan.WorktreePath)
		if err != nil {
			return "", err
		}
		if status != plan.status {
			return "", errors.New("git status in the new worktree differs from before the conversion; make sure nothing else is writing to the repository, then retry")
		}
		return "", nil
	}})
	migrate.addRollback(migrateRollbackPhaseID, migrateRollbackStepID, "Restore original layout")

	return migrate, func(phases []progress.Phase, err error) MigrateResult {
		result := MigrateResult{BareRoot: root, Phases: phases, Err: err}
		if !result.HasFailures() {
			result.Branch = plan.Branch
			result.WorktreePath = plan.WorktreePath
		}
		return result
	}
}

// Failure explains why the migration did not complete and what state the
// repository is in. It is nil when the migration succeeded.
func (r MigrateResult) Failure() error {
	if r.Err != nil {
		return fmt.Errorf("%w; nothing was deleted, but check the layout of %s before retrying", r.Err, r.BareRoot)
	}
	_, failed, ok := progress.FirstFailure(r.Phases)
	if !ok {
		return nil
	}
	if rollback := stepResult(r.Phases, migrateRollbackPhaseID, migrateRollbackStepID); rollback.Status == progress.StepFailed {
		return fmt.Errorf("%w; restoring the original layout also failed: %w. Nothing was deleted: the repository data is in .git or .bare under %s, and the files are there or in the worktree directory beside them", failed.Error, rollback.Error, r.BareRoot)
	}
	return fmt.Errorf("%w (the repository is unchanged)", failed.Error)
}

// worktreeStatus reads the status without taking the optional index lock, so
// the index file is never rewritten by a check.
func worktreeStatus(runner git.CommandRunner, path string) (string, error) {
	return runner.Run(path, "--no-optional-locks", "status", "--porcelain")
}

// renames records completed renames so they can be reversed exactly.
type renames []struct{ from, to string }

func (r *renames) move(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	*r = append(*r, struct{ from, to string }{from, to})
	return nil
}

// undo puts everything back, newest first. os.Rename replaces an existing
// file, so a path that something has written again since it was moved away is
// left alone and reported: both versions stay on disk.
func (r *renames) undo() error {
	var undoErr error
	for index := len(*r) - 1; index >= 0; index-- {
		from, to := (*r)[index].from, (*r)[index].to
		if exists(from) {
			undoErr = errors.Join(undoErr, fmt.Errorf("%s exists again, so %s was not moved back over it", from, to))
			continue
		}
		undoErr = errors.Join(undoErr, os.Rename(to, from))
	}
	return undoErr
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

// removeFile removes a file. A missing path is fine and a directory is left
// alone, so an undo can never take a directory with it.
func removeFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() {
		return nil
	}
	return os.Remove(path)
}
