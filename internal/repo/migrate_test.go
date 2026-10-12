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
// repository config and a hook.
func dirtyCheckout(t *testing.T) (base, root string) {
	t.Helper()
	base, root = newCheckout(t)
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
	hook := filepath.Join(root, ".git", "hooks", "pre-push")
	mustWrite(t, hook, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
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
		"head":         mustGit(t, worktree, "symbolic-ref", "HEAD"),
	}
}

func TestMigrate_MovesTheCheckoutIntoAWorktreeWithoutLosingAnything(t *testing.T) {
	_, root := dirtyCheckout(t)
	gitBefore := gitState(t, root)
	filesBefore := snapshot(t, root)
	for path := range filesBefore {
		if path == ".git" || strings.HasPrefix(path, ".git"+string(filepath.Separator)) {
			delete(filesBefore, path)
		}
	}
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
	filesAfter := snapshot(t, worktree)
	delete(filesAfter, ".git")
	assertSameSnapshot(t, filesBefore, filesAfter)

	// Git reports the same staged, unstaged, untracked and ignored state,
	// the same stash, reflog, branches, remotes and config.
	gitAfter := gitState(t, worktree)
	for key, before := range gitBefore {
		if gitAfter[key] != before {
			t.Errorf("%s changed:\n--- before\n%s\n--- after\n%s", key, before, gitAfter[key])
		}
	}
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
			name: "submodules",
			setup: func(t *testing.T, base, root string) string {
				library := filepath.Join(base, "library")
				mustGit(t, base, "init", "-q", "-b", "main", library)
				mustWrite(t, filepath.Join(library, "lib.txt"), "lib\n")
				mustGit(t, library, "add", ".")
				mustGit(t, library, "commit", "-qm", "lib")
				mustGit(t, root, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, "vendor/library")
				mustGit(t, root, "commit", "-qm", "add submodule")
				return root
			},
			want: "submodules",
		},
		{
			name: "sparse checkout",
			setup: func(t *testing.T, base, root string) string {
				mustGit(t, root, "sparse-checkout", "set", "src")
				return root
			},
			want: "sparse checkout or per-worktree config is enabled",
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
// rollback must hand back the original repository, byte for byte.
func TestMigrate_FailureAtAnyStepRestoresTheOriginalLayout(t *testing.T) {
	probe, _ := prepareMigrate(realGit, MigrateOptions{RepoPath: t.TempDir()})
	for index, target := range probe.steps {
		if target.id == "preflight" {
			continue
		}
		t.Run(target.id, func(t *testing.T) {
			base, root := dirtyCheckout(t)
			before := snapshot(t, base)
			gitBefore := gitState(t, root)
			migrate, finish := prepareMigrate(realGit, MigrateOptions{RepoPath: root})
			work := migrate.steps[index].run
			migrate.steps[index].run = func() (string, error) {
				if _, err := work(); err != nil {
					t.Fatalf("fixture: %s failed on its own: %v", target.id, err)
				}
				return "", errors.New("injected")
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
			for key, state := range gitState(t, root) {
				if state != gitBefore[key] {
					t.Errorf("%s changed:\n--- before\n%s\n--- after\n%s", key, gitBefore[key], state)
				}
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
