package cmd

import (
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// --merged must select the branches the default branch has absorbed, through a
// merge commit or a fast-forward, and leave alone the ones nobody has worked
// on. Branches are created from the bare repository unless a case says
// otherwise, as `sentei create` does: git writes no reflog entry for a branch
// created there, only for the commits made later in its worktree.
func TestCheckMerged_RealGit(t *testing.T) {
	bareRepo := setupBareRepo(t)
	mainPath := filepath.Join(bareRepo, "main")
	mustGit(t, bareRepo, "worktree", "add", mainPath, "main")

	// commitOn adds one commit to branch through a throwaway worktree.
	// gitOptions go before the commit subcommand.
	commitOn := func(branch string, gitOptions ...string) {
		t.Helper()
		wtPath := filepath.Join(t.TempDir(), "wt")
		mustGit(t, bareRepo, "worktree", "add", wtPath, branch)
		mustWriteFile(t, filepath.Join(wtPath, filepath.Base(branch)+".txt"), branch+"\n")
		mustGit(t, wtPath, "add", ".")
		mustGit(t, wtPath, append(gitOptions, "commit", "-m", "work on "+branch)...)
		mustGit(t, bareRepo, "worktree", "remove", "--force", wtPath)
	}

	// Every check builds its own checker, as every run of the command does.
	check := func(branch string, want bool, why string) {
		t.Helper()
		if got := CheckMerged(&git.GitRunner{}, bareRepo, "main")(branch); got != want {
			t.Errorf("isMerged(%q) = %v, want %v: %s", branch, got, want, why)
		}
	}

	mustGit(t, bareRepo, "branch", "feature/unused", "main")
	// Created from inside a worktree, so its reflog holds the creation entry.
	mustGit(t, mainPath, "branch", "feature/unused-logged", "main")
	check("feature/unused", false, "nobody has worked on it")
	check("feature/unused-logged", false, "its reflog shows only its creation")

	mustGit(t, bareRepo, "branch", "feature/unmerged", "main")
	commitOn("feature/unmerged")
	check("feature/unmerged", false, "its commit is not in main")

	mustGit(t, bareRepo, "branch", "feature/fast-forward", "main")
	commitOn("feature/fast-forward")
	mustGit(t, mainPath, "merge", "--ff-only", "feature/fast-forward")
	check("feature/fast-forward", true, "main was fast-forwarded to its commit and has not moved since")
	check("feature/unused", false, "main moving on does not make an unused branch merged")
	check("feature/unused-logged", false, "main moving on does not make an unused branch merged")

	mustGit(t, bareRepo, "branch", "feature/no-reflog", "main")
	commitOn("feature/no-reflog", "-c", "core.logAllRefUpdates=false")
	mustGit(t, mainPath, "merge", "--ff-only", "feature/no-reflog")
	check("feature/no-reflog", false, "without a reflog nothing shows that it was worked on")

	mustGit(t, bareRepo, "branch", "feature/merge-commit", "main")
	commitOn("feature/merge-commit")
	mustGit(t, mainPath, "merge", "--no-ff", "-m", "merge feature", "feature/merge-commit")
	check("feature/merge-commit", true, "main contains it through a merge commit")

	mustGit(t, bareRepo, "branch", "feature/squashed", "main")
	commitOn("feature/squashed")
	mustGit(t, mainPath, "merge", "--squash", "feature/squashed")
	mustGit(t, mainPath, "commit", "-m", "squash feature")
	check("feature/squashed", false, "a squash merge leaves its commit outside main")

	mustWriteFile(t, filepath.Join(mainPath, "later.txt"), "later\n")
	mustGit(t, mainPath, "add", ".")
	mustGit(t, mainPath, "commit", "-m", "main moves on")
	check("feature/fast-forward", true, "main has moved past the fast-forwarded branch")
	check("feature/merge-commit", true, "main has moved past the merge commit")
	check("feature/unused", false, "still nobody has worked on it")
	check("feature/unused-logged", false, "still nobody has worked on it")
	check("feature/no-reflog", false, "still no reflog to show that it was worked on")
	check("feature/unmerged", false, "its commit is still not in main")
	check("feature/squashed", false, "its commit is still not in main")
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
