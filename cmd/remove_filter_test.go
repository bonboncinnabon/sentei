package cmd

import (
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// --merged must mean "the default branch has absorbed this branch". A branch
// that still points at the default branch's tip has nothing merged from it:
// it was just created, so removing it would delete a worktree nobody has used
// yet.
func TestCheckMerged_RealGit(t *testing.T) {
	bareRepo := setupBareRepo(t)
	mainPath := filepath.Join(bareRepo, "main")
	mustGit(t, bareRepo, "worktree", "add", mainPath, "main")

	// commitOn adds one commit to branch through a throwaway worktree.
	commitOn := func(branch string) {
		t.Helper()
		wtPath := filepath.Join(t.TempDir(), "wt")
		mustGit(t, bareRepo, "worktree", "add", wtPath, branch)
		mustWriteFile(t, filepath.Join(wtPath, filepath.Base(branch)+".txt"), branch+"\n")
		mustGit(t, wtPath, "add", ".")
		mustGit(t, wtPath, "commit", "-m", "work on "+branch)
		mustGit(t, bareRepo, "worktree", "remove", "--force", wtPath)
	}

	isMerged := CheckMerged(&git.GitRunner{}, bareRepo, "main")
	check := func(branch string, want bool, why string) {
		t.Helper()
		if got := isMerged(branch); got != want {
			t.Errorf("isMerged(%q) = %v, want %v: %s", branch, got, want, why)
		}
	}

	mustGit(t, bareRepo, "branch", "feature/brand-new", "main")
	check("feature/brand-new", false, "a branch at the default tip has nothing of its own merged")

	mustGit(t, bareRepo, "branch", "feature/unmerged", "main")
	commitOn("feature/unmerged")
	check("feature/unmerged", false, "its commit is not in main")

	mustGit(t, bareRepo, "branch", "feature/merge-commit", "main")
	commitOn("feature/merge-commit")
	mustGit(t, mainPath, "merge", "--no-ff", "-m", "merge feature", "feature/merge-commit")
	check("feature/merge-commit", true, "main contains it through a merge commit")

	mustGit(t, bareRepo, "branch", "feature/fast-forward", "main")
	commitOn("feature/fast-forward")
	mustGit(t, mainPath, "merge", "--ff-only", "feature/fast-forward")
	check("feature/fast-forward", false, "right after a fast-forward it is indistinguishable from a new branch")

	mustWriteFile(t, filepath.Join(mainPath, "later.txt"), "later\n")
	mustGit(t, mainPath, "add", ".")
	mustGit(t, mainPath, "commit", "-m", "main moves on")
	check("feature/fast-forward", true, "main has moved past the fast-forwarded branch")
	check("feature/unmerged", false, "its commit is still not in main")
	check("main", false, "the default branch is never merged into itself")
	check("no/such-branch", false, "an unknown branch is not merged")
}

// A tag with the branch's name must not stand in for the branch: git resolves
// a bare name to the tag first.
func TestCheckMerged_IgnoresTagWithBranchName(t *testing.T) {
	bareRepo := setupBareRepo(t)
	mustGit(t, bareRepo, "branch", "release", "main")
	wtPath := filepath.Join(t.TempDir(), "wt")
	mustGit(t, bareRepo, "worktree", "add", wtPath, "release")
	mustWriteFile(t, filepath.Join(wtPath, "release.txt"), "unmerged\n")
	mustGit(t, wtPath, "add", ".")
	mustGit(t, wtPath, "commit", "-m", "unmerged release work")
	// The tag sits on an old commit that main has long contained.
	mustGit(t, bareRepo, "tag", "release", "main")
	mainPath := filepath.Join(bareRepo, "main")
	mustGit(t, bareRepo, "worktree", "add", mainPath, "main")
	mustWriteFile(t, filepath.Join(mainPath, "later.txt"), "later\n")
	mustGit(t, mainPath, "add", ".")
	mustGit(t, mainPath, "commit", "-m", "main moves on")

	if CheckMerged(&git.GitRunner{}, bareRepo, "main")("release") {
		t.Error("branch release has an unmerged commit; the tag of the same name must not make it look merged")
	}
}
