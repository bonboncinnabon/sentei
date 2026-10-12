package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

// setupSenteiLayout creates the layout sentei itself produces: <root>/.bare,
// a .git pointer at the root, and a main worktree beside them.
func setupSenteiLayout(t *testing.T) string {
	t.Helper()
	origin := setupBareRepo(t)
	root := filepath.Join(t.TempDir(), "proj")
	mustGit(t, filepath.Dir(origin), "clone", "--bare", origin, filepath.Join(root, ".bare"))
	mustWriteFile(t, filepath.Join(root, ".git"), "gitdir: ./.bare\n")
	mustGit(t, root, "worktree", "add", filepath.Join(root, "main"), "main")
	return root
}

// Agents run from inside a worktree. Without resolving the bare root, the new
// worktree was created nested inside the one the command ran from.
func TestRunCreate_FromInsideWorktreeCreatesSibling(t *testing.T) {
	root := setupSenteiLayout(t)
	t.Chdir(filepath.Join(root, "main"))

	var err error
	captureStdout(t, func() {
		err = RunCreate([]string{"--branch", "feature/x", "--base", "main"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(root, "feature-x")); statErr != nil {
		t.Errorf("the new worktree must sit beside main at the repo root: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "main", "feature-x")); !os.IsNotExist(statErr) {
		t.Errorf("the new worktree must not be nested inside the worktree the command ran from (stat err: %v)", statErr)
	}
}

// --copy-env used to copy nothing unless --ecosystems was also given, because
// the files were derived from the ecosystems selected for install.
func TestRunCreate_CopyEnvWorksWithoutEcosystems(t *testing.T) {
	root := setupSenteiLayout(t)
	main := filepath.Join(root, "main")
	// uv.lock makes the uv ecosystem detectable; its env file is .env.
	mustWriteFile(t, filepath.Join(main, "uv.lock"), "version = 1\n")
	mustWriteFile(t, filepath.Join(main, ".env"), "TOKEN=local\n")

	var err error
	captureStdout(t, func() {
		err = RunCreate([]string{"--branch", "feature/env", "--base", "main", "--copy-env", root})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, readErr := os.ReadFile(filepath.Join(root, "feature-env", ".env"))
	if readErr != nil {
		t.Fatalf("--copy-env without --ecosystems must still copy the env file: %v", readErr)
	}
	if string(got) != "TOKEN=local\n" {
		t.Errorf(".env = %q, want the source contents", got)
	}
}

func TestFindSource_NeverPicksTheBareEntry(t *testing.T) {
	worktrees := []git.Worktree{
		{Path: "/repo/.bare", IsBare: true},
		{Path: "/repo/trunk", Branch: "refs/heads/trunk"},
	}
	if got := findSource(worktrees); got != "/repo/trunk" {
		t.Errorf("findSource = %q, want /repo/trunk: the bare entry holds no working files", got)
	}
}

// Exit status is what a script or agent reads. A cleanup whose steps failed
// used to print a warning and exit 0.
func TestRunCleanup_FailedStepReturnsError(t *testing.T) {
	notARepo := t.TempDir()

	var err error
	captureStdout(t, func() {
		err = RunCleanup([]string{"--mode", "safe", notARepo})
	})

	if err == nil {
		t.Fatal("cleanup of a directory that is not a repository must return an error")
	}
}
