package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMigrate_ParseError(t *testing.T) {
	err := RunMigrate([]string{"--no-such-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRunMigrate_MissingRepoPath(t *testing.T) {
	err := RunMigrate(nil)
	if err == nil {
		t.Fatal("expected error for missing repo path")
	}
	if !strings.Contains(err.Error(), "repo path required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunMigrate_AlreadyBare(t *testing.T) {
	bareRepo := setupBareRepo(t)

	err := RunMigrate([]string{bareRepo})
	if err == nil {
		t.Fatal("expected error for already-bare repo")
	}
	if !strings.Contains(err.Error(), "repository is already bare") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunMigrate_NotARepo(t *testing.T) {
	dir := t.TempDir()

	err := RunMigrate([]string{dir})
	if err == nil {
		t.Fatal("expected error for non-repo path")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunMigrate_RefusalNamesTheReasonAndLeavesTheRepository(t *testing.T) {
	repoDir := setupNonBareRepo(t)
	mustGit(t, repoDir, "checkout", "--detach")

	var err error
	out := captureStdout(t, func() {
		err = RunMigrate([]string{repoDir})
	})
	if err == nil {
		t.Fatal("expected error for detached HEAD")
	}
	// The refusal is the news. The steps it kept from running are not listed.
	if !strings.Contains(out, "Check repository") || strings.Contains(out, "Move .git to .bare") {
		t.Errorf("output should show the failed check and no step after it:\n%s", out)
	}
	for _, want := range []string{"migration failed", "HEAD is detached", "the repository is unchanged"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
	if info, statErr := os.Stat(filepath.Join(repoDir, ".git")); statErr != nil || !info.IsDir() {
		t.Errorf("the refused repository must keep its .git directory (err=%v)", statErr)
	}
}

func TestRunMigrate_Success(t *testing.T) {
	repoDir := setupNonBareRepo(t)

	var err error
	out := captureStdout(t, func() {
		err = RunMigrate([]string{repoDir})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	worktree := filepath.Join(repoDir, "main")
	for _, want := range []string{"Migration complete", worktree} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// Steps are named by their labels, as create names them, not by the
	// identities the progress events carry.
	for _, want := range []string{"Validate: Check repository", "Convert: Move .git to .bare", "Move working tree"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing the step %q:\n%s", want, out)
		}
	}
	for _, identity := range []string{"migrate:convert", "move-git", "preflight"} {
		if strings.Contains(out, identity) {
			t.Errorf("output leaks the identity %q:\n%s", identity, out)
		}
	}
	if strings.Contains(strings.ToLower(out), "backup") {
		t.Errorf("migrate makes no backup and must not mention one:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(repoDir, ".bare")); statErr != nil {
		t.Errorf("expected .bare directory after migration: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(worktree, "README.md")); statErr != nil {
		t.Errorf("expected the working tree inside the worktree: %v", statErr)
	}
	if siblings, _ := filepath.Glob(repoDir + "_backup_*"); len(siblings) != 0 {
		t.Errorf("no backup directory may be left beside the repository: %v", siblings)
	}
}
