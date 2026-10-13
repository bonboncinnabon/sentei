package repo

import (
	"bytes"
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
// commit would lose the message a squash or cherry-pick prepared for it,
// ORIG_HEAD and FETCH_HEAD would stop resolving, a sparse checkout would lose
// its patterns and settings, and git would clone a submodule again instead of
// using the repository it has.
var perWorktreeFiles = []string{
	"index", filepath.Join("logs", "HEAD"),
	"ORIG_HEAD", "FETCH_HEAD", "AUTO_MERGE",
	"MERGE_MSG", "SQUASH_MSG", "COMMIT_EDITMSG",
	"config.worktree", filepath.Join("info", "sparse-checkout"), "modules",
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
	// The plan is made first because it decides which steps there are. The
	// pre-flight step reports it, so a refusal is the first thing that fails.
	plan, planErr := PlanMigrate(runner, opts.RepoPath)
	// The paths below hang off the absolute root. PlanMigrate resolves the
	// same path and refuses first if that fails.
	root, _ := filepath.Abs(opts.RepoPath)
	gitDir := filepath.Join(root, ".git")
	barePath := filepath.Join(root, ".bare")
	configPath := filepath.Join(barePath, "config")
	var gitMoved, stateMoved renames
	var originalConfig []byte // the repository config before the first git edit
	registered := false       // git created the worktree directory and its .git file
	files := treeMove{root: root, keep: []string{gitDir, barePath, plan.WorktreePath}}
	for _, kept := range plan.kept {
		files.keep = append(files.keep, filepath.Join(root, kept))
	}

	migrate := flow{name: "repository migration"}
	migrate.add(migrateValidatePhaseID, "Validate", step{id: "preflight", label: "Check repository", run: func() (string, error) {
		return plan.Branch, planErr
	}})
	if len(plan.submodules) > 0 {
		// A submodule's repository and working directory name each other by
		// relative path, and both move. A repository inside its working
		// directory holds no such path, so it travels with the tree, and git
		// puts it where the new worktree keeps its submodules afterwards.
		migrate.add(migrateConvertPhaseID, "Convert", step{
			id: "detach-submodules", label: "Detach submodule repositories",
			run: func() (string, error) {
				// Innermost first: a nested repository sits inside its parent's.
				for _, sub := range slices.Backward(plan.submodules) {
					if err := embedSubmodule(runner, root, filepath.Join(root, sub.work)); err != nil {
						return "", fmt.Errorf("moving the repository of submodule %s: %w", sub.work, err)
					}
				}
				return "", nil
			},
			undo: func() error {
				var undoErr error
				for _, sub := range plan.submodules {
					undoErr = errors.Join(undoErr, sub.restore(root))
				}
				return undoErr
			},
		})
	}
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
	if len(plan.relinks) > 0 {
		migrate.add(migrateConvertPhaseID, "Convert", step{
			id: "relink-worktrees", label: "Relink linked worktrees",
			run: func() (string, error) {
				for _, file := range plan.relinks {
					if err := os.WriteFile(file.gitfile, file.relinked, 0o644); err != nil {
						return "", fmt.Errorf("pointing a linked worktree at .bare: %w; make sure the file can be written, then retry", err)
					}
				}
				return "", nil
			},
			undo: func() error {
				var undoErr error
				for _, file := range plan.relinks {
					// A file that reads as it did needs nothing, and the one
					// that stopped the step cannot be written now either.
					if link, err := os.ReadFile(file.gitfile); err == nil && bytes.Equal(link, file.link) {
						continue
					}
					undoErr = errors.Join(undoErr, os.WriteFile(file.gitfile, file.link, 0o644))
				}
				return undoErr
			},
		})
	}
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
				// Where git copied the sparse settings into the worktree
				// itself, the originals replace the copies.
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
	// With per-worktree config every worktree reads the shared file, and a
	// shared core.bare=true would leave none of them a working tree. It goes
	// into .bare/config.worktree, which only the bare repository reads. The
	// checkout's own config.worktree moved into the worktree a step ago, so
	// that file is new.
	bareConfig := []string{"config", "core.bare", "true"}
	if plan.worktreeConfig {
		bareConfig = []string{"config", "--worktree", "core.bare", "true"}
	}
	migrate.add(migrateConvertPhaseID, "Convert", step{
		id: "mark-bare", label: "Mark repository as bare",
		run: func() (string, error) {
			_, err := runner.Run(root, bareConfig...)
			return "", err
		},
		undo: func() error {
			if plan.worktreeConfig {
				return removeFile(filepath.Join(barePath, "config.worktree"))
			}
			// The shared config goes back as a whole with the .git pointer.
			return nil
		},
	})
	if len(plan.remotes) > 0 {
		// git resolves a relative remote path from the top of the working
		// tree, which is about to be one level further down.
		replace := func(key, value, replaced string) error {
			_, err := runner.Run(root, "config", "--local", "--fixed-value", "--replace-all", key, value, replaced)
			return err
		}
		rewritten := 0
		migrate.add(migrateConvertPhaseID, "Convert", step{
			id: "remote-paths", label: "Make relative remote paths absolute",
			run: func() (string, error) {
				for _, remote := range plan.remotes {
					if err := replace(remote.key, remote.absolute, remote.relative); err != nil {
						return "", err
					}
					rewritten++
				}
				return strings.Join(plan.RelativeRemotes, ", "), nil
			},
			undo: func() error {
				// Only what was rewritten: git adds a value when the one to
				// replace is not there.
				var undoErr error
				for _, remote := range plan.remotes[:rewritten] {
					undoErr = errors.Join(undoErr, replace(remote.key, remote.relative, remote.absolute))
				}
				return undoErr
			},
		})
	}
	migrate.add(migrateConvertPhaseID, "Convert", step{
		id: "move-files", label: "Move working tree",
		run: func() (string, error) {
			if err := files.entries(root, plan.WorktreePath); err != nil {
				return "", err
			}
			return fmt.Sprintf("%d entries", len(files.moved)), nil
		},
		undo: files.undo,
	})
	if len(plan.submodules) > 0 {
		migrate.add(migrateConvertPhaseID, "Convert", step{
			id: "attach-submodules", label: "Reattach submodule repositories",
			run: func() (string, error) {
				// git moves each repository under the worktree's own modules
				// directory and writes both paths, nested submodules included.
				var tracked []string
				for _, sub := range plan.submodules {
					if sub.absorb {
						tracked = append(tracked, filepath.ToSlash(sub.work))
					}
				}
				if len(tracked) == 0 {
					return "", nil
				}
				_, err := runner.Run(plan.WorktreePath, append([]string{"submodule", "absorbgitdirs", "--"}, tracked...)...)
				return "", err
			},
			undo: func() error {
				// Whatever git absorbed goes back into its directory, so the
				// tree returns to the root as the first step left it.
				var undoErr error
				for _, sub := range slices.Backward(plan.submodules) {
					work := filepath.Join(plan.WorktreePath, sub.work)
					if info, err := os.Lstat(filepath.Join(work, ".git")); err == nil && info.Mode().IsRegular() {
						undoErr = errors.Join(undoErr, embedSubmodule(runner, root, work))
					}
				}
				return undoErr
			},
		})
	}
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
		state, err := checkoutState(runner, plan.WorktreePath, plan.kept, plan.state)
		if err != nil {
			return "", err
		}
		for index, reported := range state {
			if reported != plan.state[index] {
				return "", fmt.Errorf("%s in the new worktree differs from before the conversion; make sure nothing else is writing to the repository, then retry", checkoutProbes[index].name)
			}
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

// embedSubmodule moves the repository that a submodule's .git file points at
// into the place of that file. git reads a repository inside its working
// directory wherever the two are, which a pair of relative paths does not
// survive. root is any directory git can run in.
func embedSubmodule(runner git.CommandRunner, root, work string) error {
	gitfile := filepath.Join(work, ".git")
	_, gitDir, err := readGitfile(gitfile)
	if err != nil {
		return err
	}
	if err := os.Remove(gitfile); err != nil {
		return err
	}
	if err := os.Rename(gitDir, gitfile); err != nil {
		return err
	}
	// The setting names the working directory from where the repository
	// was. The file is named outright: git cannot open this repository
	// while the setting is wrong.
	_, err = runner.Run(root, "config", "--file", filepath.Join(gitfile, "config"), "--unset", "core.worktree")
	return err
}

// restore puts the submodule's repository, its config and the .git file back
// as the plan read them, from wherever embedSubmodule stopped.
func (s submodule) restore(root string) error {
	gitfile := filepath.Join(root, s.work, ".git")
	if info, err := os.Lstat(gitfile); err == nil && info.IsDir() {
		if err := os.WriteFile(filepath.Join(gitfile, "config"), s.config, 0o644); err != nil {
			return err
		}
		if exists(s.gitDir) {
			return fmt.Errorf("%s exists again, so %s was not moved back over it", s.gitDir, gitfile)
		}
		if err := os.Rename(gitfile, s.gitDir); err != nil {
			return err
		}
	}
	if exists(gitfile) {
		return nil
	}
	if err := os.WriteFile(gitfile, s.link, s.mode); err != nil {
		return err
	}
	// The file was created anew, and the umask may have taken bits off.
	return os.Chmod(gitfile, s.mode)
}

// treeMove moves the entries of a directory into another, except the paths
// to keep. A directory that holds a kept path stays where it is, and its
// other entries move into a new directory like it.
type treeMove struct {
	root    string   // the directory being emptied, for naming entries in errors
	keep    []string // absolute paths that stay where they are
	moved   renames
	created []string // directories made for the entries beside a kept path
}

func (m *treeMove) entries(from, to string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		source, target := filepath.Join(from, entry.Name()), filepath.Join(to, entry.Name())
		holdsKept := func(kept string) bool { return strings.HasPrefix(kept, source+string(filepath.Separator)) }
		switch {
		case slices.Contains(m.keep, source):
		case slices.ContainsFunc(m.keep, holdsKept):
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := os.Mkdir(target, info.Mode().Perm()); err != nil {
				return err
			}
			m.created = append(m.created, target)
			// The umask may have taken bits off.
			if err := os.Chmod(target, info.Mode().Perm()); err != nil {
				return err
			}
			if err := m.entries(source, target); err != nil {
				return err
			}
			// Removing a directory only works on an empty one: nothing but
			// the kept path was in the original, so no twin is needed.
			if os.Remove(target) == nil {
				m.created = m.created[:len(m.created)-1]
			}
		default:
			if err := m.moved.move(source, target); err != nil {
				name, _ := filepath.Rel(m.root, source)
				return fmt.Errorf("moving %s into the worktree: %w", name, err)
			}
		}
	}
	return nil
}

// undo moves everything back and removes the directories entries made, which
// are empty again by then.
func (m *treeMove) undo() error {
	undoErr := m.moved.undo()
	for _, dir := range slices.Backward(m.created) {
		undoErr = errors.Join(undoErr, os.Remove(dir))
	}
	return undoErr
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
