package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestRunCleanup_InvalidMode(t *testing.T) {
	err := RunCleanup([]string{"--mode", "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "invalid value for --mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCleanup_UnknownFlag(t *testing.T) {
	err := RunCleanup([]string{"--no-such-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRunCleanup_MissingMode(t *testing.T) {
	err := RunCleanup(nil)
	if err == nil {
		t.Fatal("expected error for missing mode")
	}
	if !strings.Contains(err.Error(), "missing required flag: --mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCleanup_SafeDryRun(t *testing.T) {
	bareRepo := setupBareRepo(t)

	var err error
	out := captureStdout(t, func() {
		err = RunCleanup([]string{"--mode", "safe", "--dry-run", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "(dry run)") {
		t.Errorf("expected '(dry run)' marker, got:\n%s", out)
	}
}

// A dry run changes nothing, so none of its lines may carry the done mark.
func TestRunCleanup_DryRunMarksNothingDone(t *testing.T) {
	bareRepo := setupBareRepo(t)
	stale := filepath.Join(bareRepo, "stale")
	mustGit(t, bareRepo, "worktree", "add", "-b", "feature/stale", stale, "main")
	if err := os.RemoveAll(stale); err != nil {
		t.Fatal(err)
	}

	var err error
	out := captureStdout(t, func() {
		err = RunCleanup([]string{"--mode", "safe", "--dry-run", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Would prune 1 stale worktree") {
		t.Fatalf("expected the stale worktree to be announced, got:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "✓") && strings.Contains(line, "Would") {
			t.Errorf("a would-be action is marked done: %q", line)
		}
	}
	if _, statErr := os.Stat(filepath.Join(bareRepo, "worktrees", "stale")); statErr != nil {
		t.Errorf("the dry run pruned the worktree metadata: %v", statErr)
	}
}

// Agents run from inside a worktree, where git judges "merged" against that
// worktree's branch. Cleanup acts on the whole repository, so the default
// branch is the reference whichever path names the repository.
func TestRunCleanup_WorktreePathJudgesMergedAgainstTheDefaultBranch(t *testing.T) {
	root := setupSenteiLayout(t)
	feature := filepath.Join(root, "feature")
	mustGit(t, root, "worktree", "add", "-b", "feature", feature, "main")
	mustGit(t, feature, "commit", "--allow-empty", "-m", "feature work")
	// In no worktree; merged into feature but not into main.
	mustGit(t, root, "branch", "wip", "feature")

	var err error
	captureStdout(t, func() {
		err = RunCleanup([]string{"--mode", "aggressive", feature})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !git.BranchExists(&git.GitRunner{}, root, "wip") {
		t.Error("wip is not merged into the default branch, but cleanup given a worktree path deleted it")
	}
}

func TestRunCleanupWithOpts_ReportsStepErrors(t *testing.T) {
	notARepo := t.TempDir()

	var err error
	out := captureStdout(t, func() {
		err = RunCleanupWithOpts(&cleanup.Options{Mode: cleanup.ModeSafe}, notARepo)
	})
	if err == nil {
		t.Fatal("a failed step must return an error so the exit status is non-zero")
	}
	if !strings.Contains(out, "⚠") || !strings.Contains(out, "repository") {
		t.Errorf("expected step error report, got:\n%s", out)
	}
}

func TestRunCleanupWithOpts_TipForNonWorktreeBranches(t *testing.T) {
	bareRepo := setupBareRepo(t)
	// A local branch not checked out in any worktree: safe mode leaves it and
	// should print the aggressive-mode tip.
	mustGit(t, bareRepo, "branch", "extra", "main")

	var err error
	out := captureStdout(t, func() {
		err = RunCleanupWithOpts(&cleanup.Options{Mode: cleanup.ModeSafe}, bareRepo)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "sentei cleanup --mode=aggressive") {
		t.Errorf("expected aggressive-mode tip, got:\n%s", out)
	}
}

// Task 1.4: DryRun against a real bare repo (covers the e2e path the
// mock-based cleanup package tests cannot).
func TestCleanupDryRun_RealRepo(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	// Drop the worktree but keep the branch: a real aggressive candidate.
	mustGit(t, bareRepo, "worktree", "remove", "--force", filepath.Join(bareRepo, "feature-merged-branch"))

	result, err := cleanup.DryRun(&git.GitRunner{}, bareRepo)
	if err != nil {
		t.Fatalf("DryRun() error: %v", err)
	}

	found := false
	for _, b := range result.AggressiveBranches {
		if b.Name == "feature/merged-branch" {
			found = true
			if b.LastCommitSubject == "" || b.LastCommitDate.IsZero() {
				t.Errorf("expected metadata on the candidate, got %+v", b)
			}
		}
		if b.Name == "main" {
			t.Error("the default branch must never be an aggressive candidate")
		}
	}
	if !found {
		t.Errorf("expected feature/merged-branch as an aggressive candidate, got %+v", result.AggressiveBranches)
	}
	if !result.AggressiveHasWork() {
		t.Error("AggressiveHasWork must be true")
	}
}

// The counts alone do not say which branches are gone. Both the dry run and
// the real run name every branch cleanup deletes and every one it leaves
// alone, with the reason.
func TestRunCleanup_NamesDeletedAndSkippedBranches(t *testing.T) {
	repoPath := setupBareRepo(t)
	origin := filepath.Join(t.TempDir(), "origin.git")
	mustGit(t, repoPath, "clone", "--bare", repoPath, origin)
	mustGit(t, repoPath, "remote", "add", "origin", origin)

	// commitOn gives branch a commit that exists nowhere else.
	commitOn := func(branch string) {
		t.Helper()
		wtPath := filepath.Join(t.TempDir(), "wt")
		mustGit(t, repoPath, "worktree", "add", wtPath, branch)
		mustGit(t, wtPath, "commit", "--allow-empty", "-m", "work on "+branch)
		mustGit(t, repoPath, "worktree", "remove", "--force", wtPath)
	}

	gone := []string{"gone/merged", "gone/unmerged", "gone/in-worktree"}
	for _, branch := range gone {
		mustGit(t, origin, "branch", branch, "main")
	}
	mustGit(t, repoPath, "fetch", "origin")
	for _, branch := range gone {
		mustGit(t, repoPath, "branch", "--track", branch, "origin/"+branch)
	}
	commitOn("gone/unmerged")
	mustGit(t, repoPath, "worktree", "add", filepath.Join(repoPath, "in-worktree"), "gone/in-worktree")
	for _, branch := range gone {
		mustGit(t, origin, "branch", "-D", branch)
	}
	mustGit(t, repoPath, "branch", "local/merged", "main")
	mustGit(t, repoPath, "branch", "local/unmerged", "main")
	commitOn("local/unmerged")

	branchLines := []string{
		"  gone/merged  (upstream gone)\n",
		"  local/merged  (not in any worktree)\n",
		"  gone/in-worktree  (checked out in worktree)\n",
		"  gone/unmerged  (not fully merged)\n",
		"  local/unmerged  (not fully merged)\n",
	}
	run := func(headings []string, args ...string) {
		t.Helper()
		var err error
		out := captureStdout(t, func() {
			err = RunCleanup(append(args, repoPath))
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var missing []string
		for _, want := range append(headings, branchLines...) {
			if !strings.Contains(out, want) {
				missing = append(missing, want)
			}
		}
		if len(missing) > 0 {
			t.Errorf("output is missing %q:\n%s", missing, out)
		}
	}

	runner := &git.GitRunner{}
	run([]string{"Would delete 2 branch(es):\n", "Would skip 3 branch(es):\n"}, "--mode", "aggressive", "--dry-run")
	if !git.BranchExists(runner, repoPath, "gone/merged") || !git.BranchExists(runner, repoPath, "local/merged") {
		t.Fatal("the dry run deleted a branch")
	}

	run([]string{"Deleted 2 branch(es):\n", "Skipped 3 branch(es):\n"}, "--mode", "aggressive")
	for _, branch := range []string{"gone/merged", "local/merged"} {
		if git.BranchExists(runner, repoPath, branch) {
			t.Errorf("%s is listed as deleted but still exists", branch)
		}
	}
	for _, branch := range []string{"gone/in-worktree", "gone/unmerged", "local/unmerged"} {
		if !git.BranchExists(runner, repoPath, branch) {
			t.Errorf("%s is listed as skipped but was deleted", branch)
		}
	}
}
