package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestMigrateResultErrorPropagatesContractError(t *testing.T) {
	want := errors.New("delivery")
	if got := migrateResultError(repo.MigrateResult{Err: want}); !errors.Is(got, want) {
		t.Fatalf("error = %v", got)
	}
}

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

func TestRunMigrate_FailsOnDetachedHead(t *testing.T) {
	repoDir := setupNonBareRepo(t)
	mustGit(t, repoDir, "checkout", "--detach")

	var err error
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			err = RunMigrate([]string{repoDir})
		})
	})
	if err == nil {
		t.Fatal("expected error for detached HEAD")
	}
	if !strings.Contains(err.Error(), "migration failed during Validate phase") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "detached HEAD") {
		t.Errorf("expected detached HEAD step failure on stderr, got:\n%s", stderr)
	}
}

func TestRunMigrate_SuccessWithDeleteBackup(t *testing.T) {
	repoDir := setupNonBareRepo(t)

	var err error
	out := captureStdout(t, func() {
		err = RunMigrate([]string{"--delete-backup", repoDir})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Migration complete") {
		t.Errorf("expected 'Migration complete', got:\n%s", out)
	}
	if !strings.Contains(out, "Backup deleted") {
		t.Errorf("expected 'Backup deleted', got:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(repoDir, ".bare")); statErr != nil {
		t.Errorf("expected .bare directory after migration: %v", statErr)
	}
}
