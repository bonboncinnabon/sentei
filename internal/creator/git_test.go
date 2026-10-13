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

// An existing local branch is checked out as it is. The run must say so:
// the base did not decide where the branch starts.
func TestRun_ExistingBranchIsCheckedOutAndReported(t *testing.T) {
	root := newGitRepo(t)
	gitIn(t, root, "branch", "feature/existing", "main")
	gitIn(t, filepath.Join(root, "main"), "commit", "-q", "--allow-empty", "-m", "main moves on")
	branchTip := gitIn(t, root, "rev-parse", "feature/existing")

	result := Run(&git.GitRunner{}, &git.DefaultShellRunner{}, Options{
		BranchName: "feature/existing", BaseBranch: "main", RepoPath: root,
	}, func(progress.Event) {})

	if result.HasFailures() {
		t.Fatalf("unexpected failure: %+v", result)
	}
	if result.ExistingBranch != "feature/existing" {
		t.Errorf("ExistingBranch = %q, want feature/existing for a branch that was already there", result.ExistingBranch)
	}
	message := stepByName(t, result, "Create worktree").Message
	if !strings.Contains(message, "existing branch feature/existing") || !strings.Contains(message, "base main was not applied") {
		t.Errorf("create step message = %q, want it to say the existing branch was used and the base was not", message)
	}
	if head := gitIn(t, result.WorktreePath, "rev-parse", "HEAD"); head != branchTip {
		t.Errorf("worktree HEAD = %s, want the existing branch tip %s", head, branchTip)
	}

	fresh := Run(&git.GitRunner{}, &git.DefaultShellRunner{}, Options{
		BranchName: "feature/fresh", BaseBranch: "main", RepoPath: root,
	}, func(progress.Event) {})
	if fresh.ExistingBranch != "" {
		t.Errorf("ExistingBranch = %q for a branch this run created", fresh.ExistingBranch)
	}
}

// A branch that exists only on origin is the user's branch: the worktree must
// check it out at the remote tip and track it, not start a new branch of the
// same name from the base. A second remote carrying the same branch is the
// case git's own name guessing refuses.
func TestRun_RemoteOnlyBranchIsCheckedOutTrackingOrigin(t *testing.T) {
	root := newGitRepo(t)
	seed := filepath.Join(filepath.Dir(root), "seed")
	gitIn(t, seed, "checkout", "-q", "-b", "feature/remote-only")
	gitIn(t, seed, "commit", "-q", "--allow-empty", "-m", "work that only the remote has")
	remoteTip := gitIn(t, seed, "rev-parse", "HEAD")
	gitIn(t, root, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitIn(t, root, "remote", "add", "fork", seed)
	gitIn(t, root, "fetch", "-q", "--all")
	if git.BranchExists(&git.GitRunner{}, root, "feature/remote-only") {
		t.Fatal("fixture: the branch must have no local head")
	}

	result := Run(&git.GitRunner{}, &git.DefaultShellRunner{}, Options{
		BranchName: "feature/remote-only", BaseBranch: "main", RepoPath: root,
	}, func(progress.Event) {})

	if result.HasFailures() {
		t.Fatalf("unexpected failure: %+v", result)
	}
	if head := gitIn(t, result.WorktreePath, "rev-parse", "HEAD"); head != remoteTip {
		t.Errorf("worktree HEAD = %s, want the remote tip %s, not the base", head, remoteTip)
	}
	if upstream := gitIn(t, root, "rev-parse", "--abbrev-ref", "feature/remote-only@{upstream}"); upstream != "origin/feature/remote-only" {
		t.Errorf("upstream = %q, want origin/feature/remote-only", upstream)
	}
	if result.ExistingBranch != "origin/feature/remote-only" {
		t.Errorf("ExistingBranch = %q, want origin/feature/remote-only", result.ExistingBranch)
	}
	message := stepByName(t, result, "Create worktree").Message
	if !strings.Contains(message, "remote branch origin/feature/remote-only") || !strings.Contains(message, "base main was not applied") {
		t.Errorf("create step message = %q, want it to say the remote branch was used and the base was not", message)
	}
}

// git worktree add passes the new branch name to an internal `git branch`
// call, so "-D" as the name force-deleted the branch given as the base.
func TestRun_BranchNameIsNeverParsedAsAnOption(t *testing.T) {
	root := newGitRepo(t)
	gitIn(t, root, "branch", "victim", "main")

	result := Run(&git.GitRunner{}, &git.DefaultShellRunner{}, Options{
		BranchName: "-D", BaseBranch: "victim", RepoPath: root,
	}, func(progress.Event) {})

	if result.Err == nil || !strings.Contains(result.Err.Error(), `branch name "-D"`) {
		t.Errorf("Err = %v, want the option-shaped branch name refused", result.Err)
	}
	if !git.BranchExists(&git.GitRunner{}, root, "victim") {
		t.Error("the base branch was deleted by an option-shaped branch name")
	}
}

// A base that looks like an option is refused too. Passed through, "--lock"
// made git create a locked worktree from HEAD and report success.
func TestRun_BaseIsNeverParsedAsAWorktreeOption(t *testing.T) {
	root := newGitRepo(t)

	result := Run(&git.GitRunner{}, &git.DefaultShellRunner{}, Options{
		BranchName: "feature/inject", BaseBranch: "--lock", RepoPath: root,
	}, func(progress.Event) {})

	if result.WorktreePath != "" || !result.HasFailures() {
		t.Errorf("result = %+v, want a failed run with no worktree: --lock is not a branch", result)
	}
	if listing := gitIn(t, root, "worktree", "list", "--porcelain"); strings.Contains(listing, "feature-inject") {
		t.Errorf("git created a worktree from an option-shaped base:\n%s", listing)
	}
}

// The merge step takes the same base. Passed through, "--no-ff" left git with
// no commit to merge, so it merged the branch's upstream instead.
func TestRun_BaseIsNeverParsedAsAMergeOption(t *testing.T) {
	root := newGitRepo(t)
	seed := filepath.Join(filepath.Dir(root), "seed")
	gitIn(t, seed, "commit", "-q", "--allow-empty", "-m", "upstream moves on")
	gitIn(t, root, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitIn(t, root, "fetch", "-q", "origin")
	gitIn(t, root, "branch", "--track", "feature/tracked", "origin/main")
	gitIn(t, root, "update-ref", "refs/heads/feature/tracked", "main")
	before := gitIn(t, root, "rev-parse", "feature/tracked")

	result := Run(&git.GitRunner{}, &git.DefaultShellRunner{}, Options{
		BranchName: "feature/tracked", BaseBranch: "--no-ff", RepoPath: root, MergeBase: true,
	}, func(progress.Event) {})

	if !result.HasFailures() {
		t.Errorf("result = %+v, want a failure: --no-ff is not a branch", result)
	}
	if after := gitIn(t, root, "rev-parse", "feature/tracked"); after != before {
		t.Errorf("branch moved from %s to %s: git merged something other than the base", before, after)
	}
}

// A repository with no checked-out worktree has no source for env files. The
// empty source path used to resolve against the process working directory,
// so whatever .env sat where sentei was started got copied in.
func TestRun_CopyEnvWithoutASourceWorktreeCopiesNothing(t *testing.T) {
	root := newGitRepo(t)
	bystander := t.TempDir()
	writeFile(t, filepath.Join(bystander, ".env"), "SECRET=from-the-current-directory\n")
	t.Chdir(bystander)

	result := Run(&git.GitRunner{}, &git.DefaultShellRunner{}, Options{
		BranchName: "feature/env", BaseBranch: "main", RepoPath: root,
		CopyEnvFiles: true, EnvFiles: []string{".env"}, SourceWorktree: "",
	}, func(progress.Event) {})

	if result.HasFailures() {
		t.Fatalf("unexpected failure: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(result.WorktreePath, ".env")); err == nil {
		t.Error("an env file from the current directory was copied into the new worktree")
	}
	if message := stepByName(t, result, "Copy env files").Message; !strings.Contains(message, "no source worktree") {
		t.Errorf("env step message = %q, want it to say there is no source worktree", message)
	}
}
