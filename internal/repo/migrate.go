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

const (
	migrateValidatePhaseID progress.PhaseID = "migrate:validate"
	migrateConvertPhaseID  progress.PhaseID = "migrate:convert"
	migrateRollbackPhaseID progress.PhaseID = "migrate:rollback"
	migrateRollbackStepID  progress.StepID  = "restore"
)

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
			// Every git edit to the config comes after this step, so this is
			// where the copy for the rollback is taken.
			var err error
			if originalConfig, err = os.ReadFile(configPath); err != nil {
				return "", err
			}
			return "", os.WriteFile(gitDir, []byte("gitdir: .bare\n"), 0o644)
		},
		undo: func() error {
			var undoErr error
			if originalConfig != nil {
				// The file goes back as it was read. Undoing the edits one
				// by one would not be enough: registering the worktree can
				// add an extension and raise the repository format version.
				// A read-only config cannot be written in place. git
				// replaces the file instead, as it did for its own edits.
				if undoErr = os.WriteFile(configPath, originalConfig, 0o644); undoErr != nil {
					_, undoErr = runner.Run(root, "config", "core.bare", "false")
				}
			}
			return errors.Join(undoErr, removeFile(gitDir))
		},
	})
	migrate.add(migrateConvertPhaseID, "Convert", step{
		id: "worktree", label: "Register worktree",
		run: func() (string, error) {
			// --no-checkout registers the worktree without writing a single
			// file: the existing files and index are moved in instead. The
			// repository is not marked bare yet, and git will not add a
			// worktree for the branch a main working tree has checked out,
			// so this one command is told that it is bare.
			if _, err := runner.Run(root, "-c", "core.bare=true", "worktree", "add", "--no-checkout", plan.WorktreePath, plan.Branch); err != nil {
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
	// No undo of its own: the config goes back as a whole with the .git
	// pointer.
	migrate.add(migrateConvertPhaseID, "Convert", step{id: "mark-bare", label: "Mark repository as bare", run: func() (string, error) {
		_, err := runner.Run(root, "config", "core.bare", "true")
		return "", err
	}})
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

// removeFile removes a file. A missing path is fine and a directory is left
// alone, so an undo can never take a directory with it.
func removeFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.IsDir() {
		return nil
	}
	return os.Remove(path)
}
