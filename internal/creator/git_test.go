package creator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

// newGitRepo builds the layout sentei manages (.bare, a .git pointer and a
// main worktree) in a temp dir and returns its root. Git is cut off from the
// developer's configuration for the whole test, including the runs made by
// the code under test.
func newGitRepo(t *testing.T) string {
	t.Helper()
	for name, value := range map[string]string{
		"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_SYSTEM": os.DevNull,
		"GIT_AUTHOR_NAME": "sentei-test", "GIT_AUTHOR_EMAIL": "test@sentei.invalid",
		"GIT_COMMITTER_NAME": "sentei-test", "GIT_COMMITTER_EMAIL": "test@sentei.invalid",
	} {
		t.Setenv(name, value)
	}
	tmp := t.TempDir()
	seed := filepath.Join(tmp, "seed")
	root := filepath.Join(tmp, "repo")
	gitIn(t, tmp, "init", "-q", "--initial-branch=main", seed)
	writeFile(t, filepath.Join(seed, "README.md"), "# seed\n")
	gitIn(t, seed, "add", ".")
	gitIn(t, seed, "commit", "-qm", "init")
	gitIn(t, tmp, "clone", "-q", "--bare", seed, filepath.Join(root, ".bare"))
	writeFile(t, filepath.Join(root, ".git"), "gitdir: ./.bare\n")
	gitIn(t, root, "worktree", "add", "-q", filepath.Join(root, "main"), "main")
	return root
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func stepByName(t *testing.T, result Result, name string) progress.StepResult {
	t.Helper()
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Name == name {
				return step
			}
		}
	}
	t.Fatalf("no step named %q in %+v", name, result.Phases)
	return progress.StepResult{}
}

// git worktree add exits non-zero when a post-checkout hook fails, but the
// worktree is in place by then. Treating that as "nothing created" skipped
// the rest of setup and left a worktree a second run could not get past.
func TestRun_FailedPostCheckoutHookStillSetsUpTheWorktree(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, filepath.Join(root, ".bare", "hooks", "post-checkout"), "#!/bin/sh\necho hook-says-no >&2\nexit 3\n")
	worktree := filepath.Join(root, "feature-hook")

	result := Run(&git.GitRunner{}, &git.DefaultShellRunner{}, Options{
		BranchName: "feature/hook", BaseBranch: "main", RepoPath: root, MergeBase: true,
		Ecosystems: []config.EcosystemConfig{{Name: "marker", Install: config.InstallConfig{Command: "touch installed"}}},
	}, func(progress.Event) {})

	if result.WorktreePath != worktree {
		t.Fatalf("WorktreePath = %q, want %q: the worktree exists on disk", result.WorktreePath, worktree)
	}
	create := stepByName(t, result, "Create worktree")
	if create.Status != progress.StepFailed || !strings.Contains(create.Error.Error(), "hook-says-no") {
		t.Errorf("create step = %v / %v, want failed with the hook's output", create.Status, create.Error)
	}
	if merge := stepByName(t, result, "Merge base branch"); merge.Status != progress.StepDone {
		t.Errorf("merge step status = %v (%s), want done: setup must continue in the worktree", merge.Status, merge.Message)
	}
	if _, err := os.Stat(filepath.Join(worktree, "installed")); err != nil {
		t.Errorf("the install step must run in the worktree: %v", err)
	}
	if !result.HasFailures() {
		t.Error("the failed hook must still count as a failure")
	}
}

// A directory that was there before the command is someone else's worktree:
// it must never be reported as created, or setup would run inside it.
func TestRun_ExistingWorktreeDirectoryIsNotClaimed(t *testing.T) {
	root := newGitRepo(t)
	// feature/a and feature-a map to the same directory name.
	gitIn(t, root, "worktree", "add", "-q", "-b", "feature/a", filepath.Join(root, "feature-a"), "main")

	result := Run(&git.GitRunner{}, &git.DefaultShellRunner{}, Options{
		BranchName: "feature-a", BaseBranch: "main", RepoPath: root,
		Ecosystems: []config.EcosystemConfig{{Name: "marker", Install: config.InstallConfig{Command: "touch installed"}}},
	}, func(progress.Event) {})

	if result.WorktreePath != "" {
		t.Errorf("WorktreePath = %q, want empty: nothing was created", result.WorktreePath)
	}
	if create := stepByName(t, result, "Create worktree"); create.Status != progress.StepFailed {
		t.Errorf("create step status = %v, want failed", create.Status)
	}
	if _, err := os.Stat(filepath.Join(root, "feature-a", "installed")); err == nil {
		t.Error("the install step ran inside a worktree this command did not create")
	}
}
