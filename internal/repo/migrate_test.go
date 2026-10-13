package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

var realGit = &git.GitRunner{}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := realGit.Run(dir, args...)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return out
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newCheckout creates an ordinary repository at <base>/proj with one commit
// on main and returns the base directory and the repository root.
func newCheckout(t *testing.T) (base, root string) {
	t.Helper()
	base = t.TempDir()
	root = filepath.Join(base, "proj")
	mustGit(t, base, "init", "-q", "-b", "main", root)
	mustWrite(t, filepath.Join(root, "a.txt"), "a\n")
	mustWrite(t, filepath.Join(root, ".gitignore"), "*.log\n")
	mustWrite(t, filepath.Join(root, "src", "deep", "f.txt"), "f\n")
	mustGit(t, root, "add", ".")
	mustGit(t, root, "commit", "-qm", "init")
	return base, root
}

// dirtyCheckout is newCheckout on a feature branch carrying every kind of
// state a migration could lose: a stash, staged and unstaged edits and
// deletions, untracked and ignored files, a symlink, a FIFO, an extra remote,
// repository config with an include and a hook.
func dirtyCheckout(t *testing.T) (base, root string) {
	t.Helper()
	base, root = newCheckout(t)
	dirty(t, base, root)
	return base, root
}

func dirty(t *testing.T, base, root string) {
	t.Helper()
	mustGit(t, root, "checkout", "-q", "-b", "feature/x")
	mustWrite(t, filepath.Join(root, "b.txt"), "b\n")
	mustGit(t, root, "add", ".")
	mustGit(t, root, "commit", "-qm", "feature work")

	mustWrite(t, filepath.Join(root, "a.txt"), "stashed edit\n")
	mustGit(t, root, "stash")

	mustWrite(t, filepath.Join(root, "staged.txt"), "staged\n")
	mustGit(t, root, "add", "staged.txt")
	mustGit(t, root, "rm", "-q", "--cached", "src/deep/f.txt") // staged deletion, file kept
	mustWrite(t, filepath.Join(root, "a.txt"), "a\nunstaged edit\n")
	if err := os.Remove(filepath.Join(root, "b.txt")); err != nil { // unstaged deletion
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "untracked.txt"), "untracked\n")
	mustWrite(t, filepath.Join(root, "build.log"), "ignored\n")
	mustWrite(t, filepath.Join(root, "node_modules", "pkg", "index.js"), "ignored tree\n")
	mustWrite(t, filepath.Join(root, ".git", "info", "exclude"), "node_modules/\n")
	if err := os.Symlink("a.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	// A FIFO blocks any tool that opens it for reading. Not every platform
	// has mkfifo; the rest of the fixture stands without it.
	_ = exec.Command("mkfifo", filepath.Join(root, "pipe")).Run()

	mustGit(t, root, "remote", "add", "origin", "https://example.invalid/proj.git")
	mustGit(t, root, "remote", "add", "upstream", "https://example.invalid/upstream.git")
	mustGit(t, root, "config", "sentei.test", "kept")
	// An included file outside the working tree stays where it is.
	mustWrite(t, filepath.Join(base, "shared.gitconfig"), "[sentei]\n\tincluded = kept too\n")
	mustGit(t, root, "config", "include.path", filepath.Join(base, "shared.gitconfig"))
	hook := filepath.Join(root, ".git", "hooks", "pre-push")
	mustWrite(t, hook, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
}

// commitAll commits everything in dir.
func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	mustGit(t, dir, "add", ".")
	mustGit(t, dir, "commit", "-qm", message)
}

// addSubmodules commits three submodules to the checkout: vendor/library,
// which has a submodule of its own, vendor/plain, and tools/tool, which is
// deinitialised again so that only its repository under .git/modules is left.
func addSubmodules(t *testing.T, base, root string) {
	t.Helper()
	repository := func(name string) string {
		dir := filepath.Join(base, name)
		mustGit(t, base, "init", "-q", "-b", "main", dir)
		mustWrite(t, filepath.Join(dir, name+".txt"), name+"\n")
		commitAll(t, dir, name)
		return dir
	}
	// git only clones a submodule from a local path when told it may.
	submodule := func(dir string, args ...string) {
		mustGit(t, dir, append([]string{"-c", "protocol.file.allow=always", "submodule"}, args...)...)
	}
	inner, library, plain, tool := repository("inner"), repository("library"), repository("plain"), repository("tool")
	submodule(library, "add", "-q", inner, "deps/inner")
	commitAll(t, library, "add inner")
	submodule(root, "add", "-q", library, "vendor/library")
	submodule(root, "add", "-q", plain, "vendor/plain")
	submodule(root, "add", "-q", tool, "tools/tool")
	commitAll(t, root, "add submodules")
	submodule(root, "update", "-q", "--init", "--recursive")
	submodule(root, "deinit", "-q", "-f", "tools/tool")
}

// dirtySubmodules leaves local changes in a submodule and in the submodule
// nested in it.
func dirtySubmodules(t *testing.T, root string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, "vendor", "library", "library.txt"), "edited\n")
	mustWrite(t, filepath.Join(root, "vendor", "library", "deps", "inner", "untracked.txt"), "new\n")
}

// submoduleCheckout is a checkout with submodules and uncommitted state, of
// its own and inside the submodules.
func submoduleCheckout(t *testing.T) (base, root string) {
	t.Helper()
	base, root = newCheckout(t)
	addSubmodules(t, base, root)
	dirty(t, base, root)
	dirtySubmodules(t, root)
	return base, root
}

// snapshot describes every entry under dir: type, permissions and content.
// Two equal snapshots mean nothing was added, removed, moved or rewritten.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Some development machines run git through a wrapper (git-ai) that
		// keeps its own notes in an ai directory inside the repository,
		// written behind git's back. They are no part of what a migration
		// moves or has to put back.
		if entry.IsDir() && entry.Name() == "ai" && exists(filepath.Join(filepath.Dir(path), "HEAD")) {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		switch mode := info.Mode(); {
		case mode.IsDir():
			entries[rel] = "dir"
		case mode&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			entries[rel] = "link " + target
		case mode.IsRegular():
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entries[rel] = fmt.Sprintf("file %v %q", mode.Perm(), content)
		default:
			entries[rel] = "special " + mode.Type().String()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return entries
}

func assertSameSnapshot(t *testing.T, before, after map[string]string) {
	t.Helper()
	if maps.Equal(before, after) {
		return
	}
	paths := slices.Sorted(maps.Keys(before))
	for path := range after {
		if _, ok := before[path]; !ok {
			paths = append(paths, path)
		}
	}
	for _, path := range paths {
		if before[path] != after[path] {
			t.Errorf("%s changed:\n  before: %.200s\n  after:  %.200s", path, before[path], after[path])
		}
	}
}

// gitOrNone is for questions git answers with an error when there is nothing
// to report, such as the submodules when .gitmodules does not name a gitlink.
func gitOrNone(dir string, args ...string) string {
	out, err := realGit.Run(dir, args...)
	if err != nil {
		return "none"
	}
	return out
}

// gitState is what git itself reports for a working tree. It must read the
// same through the migrated worktree as it did through the original checkout.
func gitState(t *testing.T, worktree string) map[string]string {
	t.Helper()
	return map[string]string{
		"status":       mustGit(t, worktree, "status", "--porcelain"),
		"staged diff":  mustGit(t, worktree, "diff", "--cached"),
		"working diff": mustGit(t, worktree, "diff"),
		"ignored":      mustGit(t, worktree, "status", "--porcelain", "--ignored"),
		"stash":        mustGit(t, worktree, "stash", "list"),
		"stash diff":   mustGit(t, worktree, "stash", "show", "-p"),
		"HEAD reflog":  mustGit(t, worktree, "reflog"),
		"branches":     mustGit(t, worktree, "branch", "--format=%(refname) %(objectname)"),
		"remotes":      mustGit(t, worktree, "remote", "-v"),
		"config":       mustGit(t, worktree, "config", "sentei.test"),
		"included":     mustGit(t, worktree, "config", "sentei.included"),
		"head":         mustGit(t, worktree, "symbolic-ref", "HEAD"),
		"submodules":   gitOrNone(worktree, "submodule", "status", "--recursive"),
	}
}

// assertSameGitState compares what git reports with what it reported before.
func assertSameGitState(t *testing.T, before, after map[string]string) {
	t.Helper()
	for key, state := range before {
		if after[key] != state {
			t.Errorf("%s changed:\n--- before\n%s\n--- after\n%s", key, state, after[key])
		}
	}
}

// checkoutFiles is the snapshot of a working tree without the .git entries of
// the checkout and of its submodules.
func checkoutFiles(t *testing.T, worktree string) map[string]string {
	t.Helper()
	files := snapshot(t, worktree)
	for path := range files {
		if slices.Contains(strings.Split(path, string(filepath.Separator)), ".git") {
			delete(files, path)
		}
	}
	return files
}

func TestMigrate_MovesTheCheckoutIntoAWorktreeWithoutLosingAnything(t *testing.T) {
	_, root := dirtyCheckout(t)
	gitBefore := gitState(t, root)
	filesBefore := checkoutFiles(t, root)
	hookBefore := snapshot(t, filepath.Join(root, ".git", "hooks"))

	result := Migrate(realGit, MigrateOptions{RepoPath: root}, nil)

	if err := result.Failure(); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	worktree := filepath.Join(root, "feature-x")
	if result.BareRoot != root || result.WorktreePath != worktree || result.Branch != "feature/x" {
		t.Errorf("result = root %q worktree %q branch %q", result.BareRoot, result.WorktreePath, result.Branch)
	}

	// The container holds the repository data, the pointer and the worktree.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if want := []string{".bare", ".git", "feature-x"}; !slices.Equal(names, want) {
		t.Errorf("root holds %v, want %v", names, want)
	}
	if pointer, _ := os.ReadFile(filepath.Join(root, ".git")); string(pointer) != "gitdir: .bare\n" {
		t.Errorf(".git pointer = %q", pointer)
	}
	if DetectContext(realGit, root) != ContextBareRepo || DetectContext(realGit, worktree) != ContextBareRepo {
		t.Error("the migrated repository is not detected as a bare layout")
	}

	// Every file is where it was, one directory down, byte for byte.
	assertSameSnapshot(t, filesBefore, checkoutFiles(t, worktree))

	// Git reports the same staged, unstaged, untracked and ignored state,
	// the same stash, reflog, branches, remotes and config.
	assertSameGitState(t, gitBefore, gitState(t, worktree))
	assertSameSnapshot(t, hookBefore, snapshot(t, filepath.Join(root, ".bare", "hooks")))

	// The worktree is a working one: the stash applies and a commit lands.
	mustGit(t, worktree, "stash", "drop")
	mustGit(t, worktree, "commit", "-qm", "after migrate")
	if worktrees, err := git.ListWorktrees(realGit, root); err != nil || len(worktrees) != 2 {
		t.Errorf("worktrees = %+v, err %v; want the bare entry and one worktree", worktrees, err)
	}
}

func TestMigrate_RefusalsLeaveTheRepositoryUntouched(t *testing.T) {
	conflict := func(t *testing.T, root string) {
		mustGit(t, root, "checkout", "-q", "-b", "other")
		mustWrite(t, filepath.Join(root, "a.txt"), "theirs\n")
		mustGit(t, root, "commit", "-qam", "theirs")
		mustGit(t, root, "checkout", "-q", "main")
		mustWrite(t, filepath.Join(root, "a.txt"), "ours\n")
		mustGit(t, root, "commit", "-qam", "ours")
	}
	tests := []struct {
		name string
		// setup makes the repository unfit and returns the path to migrate.
		setup func(t *testing.T, base, root string) string
		want  string
	}{
		{
			name: "a directory inside the repository",
			setup: func(t *testing.T, base, root string) string {
				return filepath.Join(root, "src")
			},
			want: "is inside a git repository, not at its root; migrate ",
		},
		{
			name: "not a repository",
			setup: func(t *testing.T, base, root string) string {
				plain := filepath.Join(base, "plain")
				mustWrite(t, filepath.Join(plain, "notes.txt"), "notes\n")
				return plain
			},
			want: "is not a git repository",
		},
		{
			name: ".git is a file",
			setup: func(t *testing.T, base, root string) string {
				linked := filepath.Join(base, "linked")
				mustGit(t, root, "worktree", "add", "-q", "-b", "linked", linked)
				return linked
			},
			want: "linked worktree or submodule",
		},
		{
			name: "existing linked worktrees",
			setup: func(t *testing.T, base, root string) string {
				mustGit(t, root, "worktree", "add", "-q", "-b", "linked", filepath.Join(base, "linked"))
				return root
			},
			want: "already has linked worktrees",
		},
		{
			name: "unborn HEAD",
			setup: func(t *testing.T, base, root string) string {
				mustGit(t, root, "checkout", "-q", "--orphan", "fresh")
				return root
			},
			want: "has no commits yet",
		},
		{
			name: "detached HEAD",
			setup: func(t *testing.T, base, root string) string {
				mustGit(t, root, "checkout", "-q", "--detach")
				return root
			},
			want: "HEAD is detached",
		},
		{
			name: "merge in progress",
			setup: func(t *testing.T, base, root string) string {
				conflict(t, root)
				if _, err := realGit.Run(root, "merge", "other"); err == nil {
					t.Fatal("fixture: the merge was meant to conflict")
				}
				return root
			},
			want: "a merge is in progress",
		},
		{
			name: "rebase in progress",
			setup: func(t *testing.T, base, root string) string {
				conflict(t, root)
				if _, err := realGit.Run(root, "rebase", "other"); err == nil {
					t.Fatal("fixture: the rebase was meant to conflict")
				}
				return root
			},
			want: "a rebase is in progress",
		},
		{
			name: "worktree directory name is taken",
			setup: func(t *testing.T, base, root string) string {
				mustGit(t, root, "checkout", "-q", "-b", "src")
				return root
			},
			want: "already exists and is where the \"src\" worktree goes",
		},
		{
			name: ".bare is taken",
			setup: func(t *testing.T, base, root string) string {
				mustWrite(t, filepath.Join(root, ".bare"), "mine\n")
				return root
			},
			want: "already has a .bare entry",
		},
		{
			name: "sparse checkout",
			setup: func(t *testing.T, base, root string) string {
				mustGit(t, root, "sparse-checkout", "set", "src")
				return root
			},
			want: "sparse checkout or per-worktree config is enabled",
		},
		{
			name: "core.worktree",
			setup: func(t *testing.T, base, root string) string {
				mustGit(t, root, "config", "core.worktree", root)
				return root
			},
			want: "core.worktree is set",
		},
		{
			name: "config included from the working tree",
			setup: func(t *testing.T, base, root string) string {
				mustWrite(t, filepath.Join(root, ".gitconfig"), "[alias]\n\tst = status\n")
				mustGit(t, root, "config", "include.path", "../.gitconfig")
				return root
			},
			want: ".gitconfig, which moves with the working tree",
		},
		{
			name: "already converted",
			setup: func(t *testing.T, base, root string) string {
				if err := Migrate(realGit, MigrateOptions{RepoPath: root}, nil).Failure(); err != nil {
					t.Fatalf("fixture: the first migration failed: %v", err)
				}
				return root
			},
			want: "already uses the bare layout",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base, root := newCheckout(t)
			target := test.setup(t, base, root)
			before := snapshot(t, base)
			var ran []string

			result := Migrate(realGit, MigrateOptions{RepoPath: target}, func(event progress.Event) {
				if event.Status == progress.StepRunning {
					ran = append(ran, event.Step)
				}
			})

			failure := result.Failure()
			if failure == nil || !strings.Contains(failure.Error(), test.want) {
				t.Fatalf("Failure() = %v, want it to mention %q", failure, test.want)
			}
			if !strings.Contains(failure.Error(), "the repository is unchanged") {
				t.Errorf("Failure() = %v, want it to say nothing changed", failure)
			}
			if want := []string{"preflight"}; !slices.Equal(ran, want) {
				t.Errorf("steps that ran = %v, want only the pre-flight", ran)
			}
			if result.WorktreePath != "" {
				t.Errorf("WorktreePath = %q for a migration that never happened", result.WorktreePath)
			}
			assertSameSnapshot(t, before, snapshot(t, base))
		})
	}
}

// Every step does its work and then reports a failure. Whatever the step, the
// rollback must hand back the original repository, byte for byte: the plain
// checkout, and the one that has every optional step in its flow.
func TestMigrate_FailureAtAnyStepRestoresTheOriginalLayout(t *testing.T) {
	fixtures := []struct {
		name  string
		build func(t *testing.T) (base, root string)
		steps []progress.StepID
	}{
		{"checkout", dirtyCheckout, []progress.StepID{
			"preflight", "move-git", "git-pointer", "worktree", "mark-bare", "move-files", "default-branch", "verify",
		}},
		{"submodules", submoduleCheckout, []progress.StepID{
			"preflight", "detach-submodules", "move-git", "git-pointer", "worktree", "mark-bare",
			"move-files", "attach-submodules", "default-branch", "verify",
		}},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			for _, stepID := range fixture.steps[1:] {
				t.Run(stepID, func(t *testing.T) {
					t.Parallel()
					base, root := fixture.build(t)
					before := snapshot(t, base)
					gitBefore := gitState(t, root)
					migrate, finish := failingAt(t, root, stepID)
					var declared []progress.StepID
					for _, step := range migrate.steps {
						declared = append(declared, step.id)
					}
					if !slices.Equal(declared, fixture.steps) {
						t.Fatalf("the flow has the steps %v, want %v", declared, fixture.steps)
					}

					result := finish(migrate.run(nil))

					failure := result.Failure()
					if failure == nil || !strings.Contains(failure.Error(), "injected") || !strings.Contains(failure.Error(), "the repository is unchanged") {
						t.Fatalf("Failure() = %v, want the injected failure reported as rolled back", failure)
					}
					if rollback := stepResult(result.Phases, migrateRollbackPhaseID, migrateRollbackStepID); rollback.Status != progress.StepDone {
						t.Errorf("rollback = %#v, want done", rollback)
					}
					assertSameSnapshot(t, before, snapshot(t, base))
					assertSameGitState(t, gitBefore, gitState(t, root))
				})
			}
		})
	}
}

// A directory without write permission cannot be renamed into a new parent.
// The move stops part-way with entries on both sides, and the rollback has to
// bring back exactly the ones that had moved.
func TestMigrate_UnmovableEntryRollsBackThePartialMove(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	base, root := dirtyCheckout(t)
	locked := filepath.Join(root, "zz-locked")
	mustWrite(t, filepath.Join(locked, "cache.bin"), "cache\n")
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	before := snapshot(t, base)

	result := Migrate(realGit, MigrateOptions{RepoPath: root}, nil)

	failure := result.Failure()
	if failure == nil || !strings.Contains(failure.Error(), "moving zz-locked into the worktree") {
		t.Fatalf("Failure() = %v, want the unmovable entry named", failure)
	}
	if !strings.Contains(failure.Error(), "the repository is unchanged") {
		t.Errorf("Failure() = %v, want it reported as rolled back", failure)
	}
	if moved := stepResult(result.Phases, migrateConvertPhaseID, "move-files"); moved.Status != progress.StepFailed {
		t.Errorf("move-files = %#v, want failed", moved)
	}
	assertSameSnapshot(t, before, snapshot(t, base))
}

func TestMigrate_BrokenEventStreamStillRollsBack(t *testing.T) {
	base, root := dirtyCheckout(t)
	before := snapshot(t, base)
	delivery := errors.New("delivery")

	result := Migrate(realGit, MigrateOptions{RepoPath: root}, func(event progress.Event) {
		if event.Step == "move-files" && event.Status == progress.StepDone {
			panic(delivery)
		}
	})

	if !errors.Is(result.Err, delivery) || result.Failure() == nil {
		t.Fatalf("Err = %v, want the delivery failure", result.Err)
	}
	assertSameSnapshot(t, before, snapshot(t, base))
}

// Besides the index, a checkout keeps a few files of its own in .git: the
// message a squash or cherry-pick prepared for the next commit, ORIG_HEAD, the
// shared half of a split index. A linked worktree reads them from its own
// admin directory, so they have to move there with the index.
func TestMigrate_CarriesTheCheckoutsOwnGitStateIntoTheWorktree(t *testing.T) {
	_, root := newCheckout(t)
	mustGit(t, root, "checkout", "-q", "-b", "other")
	mustWrite(t, filepath.Join(root, "other.txt"), "other\n")
	mustGit(t, root, "add", ".")
	mustGit(t, root, "commit", "-qm", "work to squash")
	mustGit(t, root, "checkout", "-q", "main")
	mustGit(t, root, "update-index", "--split-index")
	mustGit(t, root, "merge", "-q", "--squash", "other")
	origHead := mustGit(t, root, "rev-parse", "ORIG_HEAD")

	result := Migrate(realGit, MigrateOptions{RepoPath: root}, nil)

	if err := result.Failure(); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	worktree := result.WorktreePath
	if got := mustGit(t, worktree, "rev-parse", "ORIG_HEAD"); got != origHead {
		t.Errorf("ORIG_HEAD = %q, want %q", got, origHead)
	}
	// The squash left its message for the next commit to pick up.
	mustGit(t, worktree, "commit", "-q", "--no-edit")
	if message := mustGit(t, worktree, "log", "-1", "--format=%B"); !strings.Contains(message, "work to squash") {
		t.Errorf("the squash commit lost its prepared message:\n%s", message)
	}
}

// The last step compares git status in the new worktree with the status taken
// before the conversion. A difference means the working tree did not arrive
// as it left, so the conversion must not stand.
func TestMigrate_StatusThatChangedDuringTheConversionRollsBack(t *testing.T) {
	_, root := newCheckout(t)
	migrate, finish := prepareMigrate(realGit, MigrateOptions{RepoPath: root})
	for index, declared := range migrate.steps {
		if declared.id != "move-files" {
			continue
		}
		move := declared.run
		migrate.steps[index].run = func() (string, error) {
			message, err := move()
			// Something edits a tracked file while the conversion is under way.
			mustWrite(t, filepath.Join(root, "main", "a.txt"), "edited under the migration\n")
			return message, err
		}
	}

	result := finish(migrate.run(nil))

	failure := result.Failure()
	if failure == nil || !strings.Contains(failure.Error(), "differs from before the conversion") {
		t.Fatalf("Failure() = %v, want the status difference reported", failure)
	}
	if !strings.Contains(failure.Error(), "the repository is unchanged") {
		t.Errorf("Failure() = %v, want it reported as rolled back", failure)
	}
	if verify := stepResult(result.Phases, migrateConvertPhaseID, "verify"); verify.Status != progress.StepFailed {
		t.Errorf("verify = %#v, want failed", verify)
	}
	// The original layout is back, and the edit is where its author left it.
	if info, err := os.Stat(filepath.Join(root, ".git")); err != nil || !info.IsDir() {
		t.Errorf(".git is not a directory again (err %v)", err)
	}
	if exists(filepath.Join(root, ".bare")) || exists(filepath.Join(root, "main")) {
		t.Error("the rollback left .bare or the worktree directory behind")
	}
	if status := mustGit(t, root, "status", "--porcelain"); status != "M a.txt" {
		t.Errorf("status = %q, want only the edit to a.txt", status)
	}
}

func TestMigrate_ResolvesRelativeRepoPath(t *testing.T) {
	base, root := newCheckout(t)
	t.Chdir(base)

	result := Migrate(realGit, MigrateOptions{RepoPath: "proj/"}, nil)

	if err := result.Failure(); err != nil {
		t.Fatal(err)
	}
	// t.Chdir resolves symlinks, so compare against the resolved root.
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := filepath.EvalSymlinks(result.BareRoot); got != want || !filepath.IsAbs(result.BareRoot) {
		t.Errorf("BareRoot = %q, want the absolute %q", result.BareRoot, want)
	}
	if _, err := os.Stat(filepath.Join(root, "main", "a.txt")); err != nil {
		t.Errorf("the worktree did not land under the repository root: %v", err)
	}
}

func TestMigrateResult_FailureNamesAFailedRollback(t *testing.T) {
	result := MigrateResult{BareRoot: "/repo", Phases: []progress.Phase{
		{ID: migrateConvertPhaseID, Steps: []progress.StepResult{{ID: "move-files", Status: progress.StepFailed, Error: errors.New("disk full")}}},
		{ID: migrateRollbackPhaseID, Steps: []progress.StepResult{{ID: migrateRollbackStepID, Status: progress.StepFailed, Error: errors.New("still full")}}},
	}}

	failure := result.Failure()

	if failure == nil {
		t.Fatal("Failure() = nil")
	}
	for _, want := range []string{"disk full", "restoring the original layout also failed", "still full", "Nothing was deleted", "/repo"} {
		if !strings.Contains(failure.Error(), want) {
			t.Errorf("Failure() = %v, want it to mention %q", failure, want)
		}
	}
	if strings.Contains(failure.Error(), "unchanged") {
		t.Errorf("Failure() = %v, must not claim the repository is unchanged", failure)
	}
}

// os.Rename replaces an existing file. If something writes a path again after
// it was moved away, the rollback must not put the old file on top of it.
func TestRenames_UndoDoesNotOverwriteAPathThatReappeared(t *testing.T) {
	dir := t.TempDir()
	kept, reappeared := filepath.Join(dir, "kept.txt"), filepath.Join(dir, "reappeared.txt")
	mustWrite(t, kept, "kept\n")
	mustWrite(t, reappeared, "original\n")
	var moved renames
	for _, path := range []string{kept, reappeared} {
		if err := moved.move(path, path+".moved"); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, reappeared, "written since\n")

	err := moved.undo()

	if err == nil || !strings.Contains(err.Error(), reappeared) {
		t.Fatalf("undo() = %v, want it to name the path it would have overwritten", err)
	}
	for path, want := range map[string]string{
		reappeared:            "written since\n",
		reappeared + ".moved": "original\n",
		kept:                  "kept\n", // the other renames are still undone
	} {
		if content, _ := os.ReadFile(path); string(content) != want {
			t.Errorf("%s = %q, want %q", path, content, want)
		}
	}
}

// The bare HEAD names the default branch: sentei protects it and measures
// "merged" against it. After a migration it must not simply be whatever
// branch happened to be checked out.
func TestMigrate_PointsBareHEADAtTheDefaultBranch(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  string
	}{
		{
			name: "main, when migrating from a feature branch",
			setup: func(t *testing.T, root string) {
				mustGit(t, root, "checkout", "-q", "-b", "feature/x")
			},
			want: "main",
		},
		{
			name: "the remote's default, ahead of main",
			setup: func(t *testing.T, root string) {
				mustGit(t, root, "branch", "trunk")
				mustGit(t, root, "update-ref", "refs/remotes/origin/trunk", "HEAD")
				mustGit(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
				mustGit(t, root, "checkout", "-q", "-b", "feature/x")
			},
			want: "trunk",
		},
		{
			name: "the current branch, when nothing names another",
			setup: func(t *testing.T, root string) {
				mustGit(t, root, "branch", "-m", "main", "develop")
				// The remote's default has no local branch to point at.
				mustGit(t, root, "update-ref", "refs/remotes/origin/trunk", "HEAD")
				mustGit(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
			},
			want: "develop",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, root := newCheckout(t)
			test.setup(t, root)
			current := mustGit(t, root, "branch", "--show-current")

			result := Migrate(realGit, MigrateOptions{RepoPath: root}, nil)

			if err := result.Failure(); err != nil {
				t.Fatal(err)
			}
			if got := git.DetectDefaultBranch(realGit, root); got != test.want {
				t.Errorf("default branch = %q, want %q", got, test.want)
			}
			if got := mustGit(t, result.WorktreePath, "branch", "--show-current"); got != current {
				t.Errorf("the worktree is on %q, want it left on %q", got, current)
			}
			// The fixture has no remote, and converting must not invent one.
			if remotes := mustGit(t, root, "remote"); remotes != "" {
				t.Errorf("remotes = %q, want none", remotes)
			}
		})
	}
}

// From a subdirectory the refusal has to say where the root is: in the TUI
// there is no path argument to correct.
func TestPlanMigrate_NamesTheRootFromASubdirectory(t *testing.T) {
	_, root := newCheckout(t)
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	_, err = PlanMigrate(realGit, filepath.Join(root, "src", "deep"))

	if err == nil || !strings.Contains(err.Error(), "migrate "+resolved+" instead") {
		t.Fatalf("err = %v, want it to point at %s", err, resolved)
	}
}

// If git refuses to register the worktree, whatever sits at that path is not
// the migration's to clean up.
func TestMigrate_FailedWorktreeAddLeavesTheDirectoryAlone(t *testing.T) {
	base, root := newCheckout(t)
	before := snapshot(t, base)
	migrate, finish := prepareMigrate(realGit, MigrateOptions{RepoPath: root})
	squatter := filepath.Join(root, "main")
	for index, declared := range migrate.steps {
		if declared.id != "worktree" {
			continue
		}
		register := declared.run
		migrate.steps[index].run = func() (string, error) {
			// Something else takes the path after the pre-flight cleared it.
			mustWrite(t, filepath.Join(squatter, ".git"), "gitdir: /elsewhere\n")
			mustWrite(t, filepath.Join(squatter, "theirs.txt"), "theirs\n")
			return register()
		}
	}

	result := finish(migrate.run(nil))

	if result.Failure() == nil {
		t.Fatal("git was meant to refuse a directory that is not empty")
	}
	for _, name := range []string{".git", "theirs.txt"} {
		if _, err := os.Stat(filepath.Join(squatter, name)); err != nil {
			t.Errorf("the rollback removed %s from a directory git never created: %v", name, err)
		}
	}
	if err := os.RemoveAll(squatter); err != nil {
		t.Fatal(err)
	}
	assertSameSnapshot(t, before, snapshot(t, base))
}

// failingAt returns the migrate flow for root with the named step doing its
// work and then reporting a failure.
func failingAt(t *testing.T, root, stepID string) (flow, func([]progress.Phase, error) MigrateResult) {
	t.Helper()
	migrate, finish := prepareMigrate(realGit, MigrateOptions{RepoPath: root})
	for index, declared := range migrate.steps {
		if declared.id != stepID {
			continue
		}
		migrate.steps[index].run = func() (string, error) {
			if _, err := declared.run(); err != nil {
				t.Fatalf("fixture: %s failed on its own: %v", stepID, err)
			}
			return "", errors.New("injected")
		}
		return migrate, finish
	}
	t.Fatalf("fixture: the migrate flow has no step %q", stepID)
	return migrate, finish
}

// gitConfigEnv sets one git config value for every git process the test
// starts, the way a user's global config would.
func gitConfigEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", key)
	t.Setenv("GIT_CONFIG_VALUE_0", value)
}

// Registering the worktree can make git rewrite more of the config than
// core.bare: with relative worktree paths it adds an extension and raises
// the repository format version, which older git versions refuse to read.
// The rollback has to take that back too.
func TestMigrate_RollbackRestoresTheConfigGitRewrote(t *testing.T) {
	gitConfigEnv(t, "worktree.useRelativePaths", "true")
	base, root := dirtyCheckout(t)
	before := snapshot(t, base)
	migrate, finish := failingAt(t, root, "move-files")

	result := finish(migrate.run(nil))

	if failure := result.Failure(); failure == nil || !strings.Contains(failure.Error(), "the repository is unchanged") {
		t.Fatalf("Failure() = %v, want the failure reported as rolled back", failure)
	}
	assertSameSnapshot(t, before, snapshot(t, base))
}

// git edits a read-only config by replacing the file. The rollback has to
// get core.bare back out of it all the same.
func TestMigrate_RollbackRestoresAReadOnlyConfig(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	base, root := dirtyCheckout(t)
	if err := os.Chmod(filepath.Join(root, ".git", "config"), 0o444); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, base)
	migrate, finish := failingAt(t, root, "move-files")

	result := finish(migrate.run(nil))

	if failure := result.Failure(); failure == nil || !strings.Contains(failure.Error(), "the repository is unchanged") {
		t.Fatalf("Failure() = %v, want the failure reported as rolled back", failure)
	}
	assertSameSnapshot(t, before, snapshot(t, base))
}

// With safe.bareRepository=explicit git refuses a bare repository it finds
// on its own, so every command has to reach .bare through the .git pointer.
func TestMigrate_WorksWhenBareRepositoriesMustBeExplicit(t *testing.T) {
	gitConfigEnv(t, "safe.bareRepository", "explicit")

	t.Run("converts", func(t *testing.T) {
		_, root := dirtyCheckout(t)
		status := mustGit(t, root, "status", "--porcelain")

		result := Migrate(realGit, MigrateOptions{RepoPath: root}, nil)

		if err := result.Failure(); err != nil {
			t.Fatalf("migrate failed: %v", err)
		}
		if got := mustGit(t, result.WorktreePath, "status", "--porcelain"); got != status {
			t.Errorf("status = %q, want %q", got, status)
		}
		if got := git.DetectDefaultBranch(realGit, root); got != "main" {
			t.Errorf("default branch = %q, want main", got)
		}
	})

	t.Run("rolls back", func(t *testing.T) {
		base, root := dirtyCheckout(t)
		before := snapshot(t, base)
		migrate, finish := failingAt(t, root, "verify")

		result := finish(migrate.run(nil))

		if failure := result.Failure(); failure == nil || !strings.Contains(failure.Error(), "the repository is unchanged") {
			t.Fatalf("Failure() = %v, want the failure reported as rolled back", failure)
		}
		assertSameSnapshot(t, before, snapshot(t, base))
	})
}

// If the config cannot be read there is nothing to put back, and the undo
// must not write an empty file in its place.
func TestMigrate_RollbackWritesNoConfigItNeverRead(t *testing.T) {
	_, root := newCheckout(t)
	config := filepath.Join(root, ".bare", "config")
	migrate, finish := prepareMigrate(realGit, MigrateOptions{RepoPath: root})
	for index, declared := range migrate.steps {
		if declared.id != "git-pointer" {
			continue
		}
		migrate.steps[index].run = func() (string, error) {
			// The config disappears just before the step that reads it.
			if err := os.Rename(config, config+".away"); err != nil {
				t.Fatal(err)
			}
			return declared.run()
		}
	}

	result := finish(migrate.run(nil))

	if pointer := stepResult(result.Phases, migrateConvertPhaseID, "git-pointer"); pointer.Status != progress.StepFailed {
		t.Fatalf("git-pointer = %#v, want failed on the missing config", pointer)
	}
	if exists(filepath.Join(root, ".git", "config")) {
		t.Error("the rollback wrote a config file it had never read")
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "config.away")); err != nil {
		t.Errorf(".git did not return with the original config: %v", err)
	}
}

// The shared half of a split index is found by name. Brackets in the
// repository path must not be read as a pattern while looking for it.
func TestMigrate_SplitIndexUnderAPathWithPatternCharacters(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "proj[1]")
	mustGit(t, base, "init", "-q", "-b", "main", root)
	mustWrite(t, filepath.Join(root, "a.txt"), "a\n")
	mustGit(t, root, "add", ".")
	mustGit(t, root, "commit", "-qm", "init")
	mustGit(t, root, "update-index", "--split-index")
	mustWrite(t, filepath.Join(root, "a.txt"), "staged edit\n")
	mustGit(t, root, "add", "a.txt")
	status := mustGit(t, root, "status", "--porcelain")

	result := Migrate(realGit, MigrateOptions{RepoPath: root}, nil)

	if err := result.Failure(); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if got := mustGit(t, result.WorktreePath, "status", "--porcelain"); got != status {
		t.Errorf("status = %q, want %q", got, status)
	}
}

// Submodules keep their checked-out commits and local changes. Their
// repositories end up where git keeps them for a linked worktree, nested ones
// included, and a repository kept for a deinitialised submodule is still used.
func TestMigrate_CarriesSubmodulesIntoTheWorktree(t *testing.T) {
	t.Parallel()
	_, root := submoduleCheckout(t)
	gitBefore := gitState(t, root)
	filesBefore := checkoutFiles(t, root)
	if !strings.Contains(gitBefore["submodules"], "vendor/library/deps/inner") || gitBefore["status"] == "" {
		t.Fatalf("fixture: want a nested submodule and local changes, got\n%s\n%s", gitBefore["submodules"], gitBefore["status"])
	}

	result := Migrate(realGit, MigrateOptions{RepoPath: root}, nil)

	if err := result.Failure(); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	worktree := result.WorktreePath
	assertSameSnapshot(t, filesBefore, checkoutFiles(t, worktree))
	assertSameGitState(t, gitBefore, gitState(t, worktree))

	modules := filepath.Join(mustGit(t, worktree, "rev-parse", "--absolute-git-dir"), "modules")
	for _, path := range []string{"vendor/library", "vendor/library/deps/inner", "vendor/plain"} {
		dir := filepath.Join(worktree, path)
		if info, err := os.Lstat(filepath.Join(dir, ".git")); err != nil || !info.Mode().IsRegular() {
			t.Errorf("%s: .git is not a file again (err %v)", path, err)
		}
		if gitDir := mustGit(t, dir, "rev-parse", "--absolute-git-dir"); !git.PathContains(modules, gitDir) {
			t.Errorf("%s: repository at %s, want it under %s", path, gitDir, modules)
		}
	}
	if exists(filepath.Join(root, ".bare", "modules")) {
		t.Error(".bare/modules was left behind, where no worktree looks for it")
	}

	// The submodules are working ones: a commit lands, and the superproject
	// sees it.
	mustGit(t, filepath.Join(worktree, "vendor", "library"), "commit", "-qam", "edit in the submodule")
	if status := mustGit(t, worktree, "status", "--porcelain", "vendor/library"); !strings.HasPrefix(status, "M ") {
		t.Errorf("status of vendor/library = %q, want it modified", status)
	}
	// The deinitialised one comes back from the repository git kept for it:
	// cloning from a path is not allowed here.
	mustGit(t, worktree, "-c", "protocol.file.allow=never", "submodule", "update", "-q", "--init", "tools/tool")
	if !exists(filepath.Join(worktree, "tools", "tool", "tool.txt")) {
		t.Error("tools/tool was not checked out again")
	}
}

// git absorbs the submodules the superproject tracks by name. A repository
// added without one, and one left behind by a branch that does not have it,
// keep their repository inside them and work as before.
func TestMigrate_LeavesRepositoriesGitDoesNotAbsorbInPlace(t *testing.T) {
	t.Parallel()
	base, root := newCheckout(t)
	addSubmodules(t, base, root)
	embedded := filepath.Join(root, "embedded")
	mustGit(t, root, "init", "-q", "-b", "main", embedded)
	mustWrite(t, filepath.Join(embedded, "e.txt"), "e\n")
	commitAll(t, embedded, "embedded")
	commitAll(t, root, "a repository without an entry in .gitmodules")
	mustGit(t, root, "checkout", "-q", "-b", "without")
	mustGit(t, root, "rm", "-q", "--cached", "vendor/plain")
	mustGit(t, root, "commit", "-qm", "a branch without vendor/plain")
	status := mustGit(t, root, "status", "--porcelain")
	if status != "?? vendor/plain/" {
		t.Fatalf("fixture: status = %q, want vendor/plain left behind untracked", status)
	}

	result := Migrate(realGit, MigrateOptions{RepoPath: root}, nil)

	if err := result.Failure(); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	worktree := result.WorktreePath
	if got := mustGit(t, worktree, "status", "--porcelain"); got != status {
		t.Errorf("status = %q, want %q", got, status)
	}
	for path, wantDirectory := range map[string]bool{"embedded": true, "vendor/plain": true, "vendor/library": false} {
		info, err := os.Lstat(filepath.Join(worktree, path, ".git"))
		if err != nil || info.IsDir() != wantDirectory {
			t.Errorf("%s: .git is a directory = %v (err %v), want %v", path, err == nil && info.IsDir(), err, wantDirectory)
		}
		if got := mustGit(t, filepath.Join(worktree, path), "status", "--porcelain"); got != "" {
			t.Errorf("%s: status = %q, want it clean", path, got)
		}
	}
}

// A submodule directory that cannot be written keeps its .git file. That
// stops the conversion after the repositories of other submodules have moved
// into their directories, and they have to come back.
func TestMigrate_SubmoduleThatCannotBeDetachedRollsBackTheOthers(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	base, root := newCheckout(t)
	addSubmodules(t, base, root)
	library := filepath.Join(root, "vendor", "library")
	if err := os.Chmod(library, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(library, 0o755) })
	before := snapshot(t, base)

	result := Migrate(realGit, MigrateOptions{RepoPath: root}, nil)

	failure := result.Failure()
	if failure == nil || !strings.Contains(failure.Error(), "moving the repository of submodule "+filepath.Join("vendor", "library")) {
		t.Fatalf("Failure() = %v, want the submodule named", failure)
	}
	if !strings.Contains(failure.Error(), "the repository is unchanged") {
		t.Errorf("Failure() = %v, want it reported as rolled back", failure)
	}
	assertSameSnapshot(t, before, snapshot(t, base))
}

// gitFunc stands in for git where a test needs one command to go wrong.
type gitFunc func(dir string, args ...string) (string, error)

func (run gitFunc) Run(dir string, args ...string) (string, error) { return run(dir, args...) }

// If git stops between two submodules, one repository is in the worktree's
// modules directory and the other still inside its submodule. The rollback
// has to return both.
func TestMigrate_InterruptedSubmoduleAbsorbRollsBack(t *testing.T) {
	t.Parallel()
	base, root := newCheckout(t)
	addSubmodules(t, base, root)
	dirtySubmodules(t, root)
	before := snapshot(t, base)
	absorbOne := gitFunc(func(dir string, args ...string) (string, error) {
		if len(args) > 4 && args[1] == "absorbgitdirs" {
			if _, err := realGit.Run(dir, args[:4]...); err != nil {
				return "", err
			}
			return "", errors.New("interrupted")
		}
		return realGit.Run(dir, args...)
	})

	result := Migrate(absorbOne, MigrateOptions{RepoPath: root}, nil)

	failure := result.Failure()
	if failure == nil || !strings.Contains(failure.Error(), "interrupted") || !strings.Contains(failure.Error(), "the repository is unchanged") {
		t.Fatalf("Failure() = %v, want the interruption reported as rolled back", failure)
	}
	assertSameSnapshot(t, before, snapshot(t, base))
}
