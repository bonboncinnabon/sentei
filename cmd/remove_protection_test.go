package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

// TestRemove_DefaultBranchProtectedFromWorktree builds a real sentei bare repo
// whose default branch is non-standard ("production") and verifies that, even
// when detection starts from inside a different worktree, the default branch is
// resolved correctly and protected from removal (regression for the worktree-
// local HEAD bug).
func TestRemove_DefaultBranchProtectedFromWorktree(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(root, ".bare")

	mustGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = testtmp.HermeticGitEnv()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	mustGit(root, "init", "--bare", "--initial-branch=production", ".bare")
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: .bare\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Seed a commit on production via a temporary clone, then push.
	mustGit(root, "clone", bare, "_seed")
	seed := filepath.Join(root, "_seed")
	mustGit(seed, "config", "user.email", "t@t.com")
	mustGit(seed, "config", "user.name", "t")
	mustGit(seed, "checkout", "-b", "production")
	if err := os.WriteFile(filepath.Join(seed, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(seed, "add", "-A")
	mustGit(seed, "commit", "-m", "init")
	mustGit(seed, "push", "origin", "production")
	os.RemoveAll(seed)

	mustGit(bare, "worktree", "add", filepath.Join(root, "production"), "production")
	mustGit(bare, "worktree", "add", "-b", "feature", filepath.Join(root, "feature"), "production")

	runner := &git.GitRunner{}

	// Start detection from INSIDE the feature worktree (the bug case): without the
	// fix, HEAD there is "feature", so "production" would be left unprotected.
	bareRoot := repo.ResolveBareRoot(runner, filepath.Join(root, "feature"))
	def := git.DetectDefaultBranch(runner, bareRoot)
	if def != "production" {
		t.Fatalf("default branch detected from a worktree = %q, want %q", def, "production")
	}

	worktrees, err := git.ListWorktrees(runner, bareRoot)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	filtered := ResolveFilters(worktrees, &RemoveOptions{All: true}, nil, def, nil)

	var hasProduction, hasFeature bool
	for _, wt := range filtered {
		switch shortBranch(wt.Branch) {
		case "production":
			hasProduction = true
		case "feature":
			hasFeature = true
		}
	}
	if hasProduction {
		t.Error("the production (default) worktree must be protected from removal")
	}
	if !hasFeature {
		t.Error("the feature worktree should be removable")
	}
}
