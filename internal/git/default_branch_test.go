package git

import (
	"path/filepath"
	"testing"
)

func TestDetectDefaultBranch_ReadsTheSharedHead(t *testing.T) {
	runner := &GitRunner{}
	root := t.TempDir()
	bare := filepath.Join(root, ".bare")
	seed := filepath.Join(root, "seed")
	feature := filepath.Join(root, "feature")
	run := func(dir string, args ...string) {
		t.Helper()
		if _, err := runner.Run(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	run(root, "init", "--quiet", "--bare", "--initial-branch=trunk", bare)
	run(root, "clone", "--quiet", bare, seed)
	run(seed, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "--quiet", "--allow-empty", "-m", "init")
	run(seed, "push", "--quiet", "origin", "HEAD:trunk")
	run(bare, "worktree", "add", "--quiet", "-b", "feature/x", feature, "trunk")

	// In a worktree HEAD is that worktree's branch. Detecting from it named
	// feature/x as the default and left trunk unprotected.
	t.Run("from a worktree on another branch", func(t *testing.T) {
		if got := DetectDefaultBranch(runner, feature); got != "trunk" {
			t.Errorf("DetectDefaultBranch(worktree) = %q, want trunk", got)
		}
	})

	// Under safe.bareRepository=explicit git refuses a bare directory it is
	// only pointed into. Detection then fell back to "main" and left trunk
	// unprotected.
	t.Run("bare repositories must be named explicitly", func(t *testing.T) {
		strict := &GitRunner{Env: []string{
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=safe.bareRepository",
			"GIT_CONFIG_VALUE_0=explicit",
		}}
		if _, err := strict.Run(bare, "symbolic-ref", "HEAD"); err == nil {
			t.Fatal("control failed: git should refuse the bare directory under safe.bareRepository=explicit")
		}
		if got := DetectDefaultBranch(strict, feature); got != "trunk" {
			t.Errorf("DetectDefaultBranch = %q, want trunk", got)
		}
	})

	// With a tag of the same name, the short form of HEAD is "heads/trunk",
	// which names no branch.
	t.Run("a tag shares the branch's name", func(t *testing.T) {
		run(bare, "tag", "trunk", "trunk")
		if got := DetectDefaultBranch(runner, bare); got != "trunk" {
			t.Errorf("DetectDefaultBranch = %q, want trunk", got)
		}
	})
}
