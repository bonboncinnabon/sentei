package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatStaleDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"one day", 24 * time.Hour, "1d"},
		{"two weeks", 14 * 24 * time.Hour, "14d"},
		{"thirty days", 30 * 24 * time.Hour, "30d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatStaleDuration(tt.d); got != tt.want {
				t.Errorf("FormatStaleDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestFormatFilterLabel(t *testing.T) {
	tests := []struct {
		name string
		opts RemoveOptions
		want string
	}{
		{"all wins over other filters", RemoveOptions{All: true, Merged: true}, "all"},
		{"merged only", RemoveOptions{Merged: true}, "merged"},
		{"stale only", RemoveOptions{Stale: 48 * time.Hour}, "stale > 2d"},
		{"merged and stale", RemoveOptions{Merged: true, Stale: 48 * time.Hour}, "merged, stale > 2d"},
		{"no filters", RemoveOptions{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatFilterLabel(&tt.opts); got != tt.want {
				t.Errorf("FormatFilterLabel(%+v) = %q, want %q", tt.opts, got, tt.want)
			}
		})
	}
}

func TestRunRemove_ParseError(t *testing.T) {
	err := RunRemove([]string{"--stale", "abc"})
	if err == nil {
		t.Fatal("expected error for invalid stale duration")
	}
	if !strings.Contains(err.Error(), "invalid duration") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunRemove_UnknownFlag(t *testing.T) {
	err := RunRemove([]string{"--no-such-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRunRemove_NoFilters(t *testing.T) {
	err := RunRemove(nil)
	if err == nil {
		t.Fatal("expected error for missing filters")
	}
	if !strings.Contains(err.Error(), "at least one filter required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunRemove_RequiresBareRepo(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		detected string
	}{
		{"not a repository", t.TempDir(), "detected: no repository"},
		{"regular repository", setupNonBareRepo(t), "detected: regular repository"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RunRemove([]string{"--all", tt.dir})
			if err == nil {
				t.Fatal("expected error for non-bare path")
			}
			for _, want := range []string{"remove requires a bare repository", tt.detected} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q missing %q", err, want)
				}
			}
		})
	}
}

func TestRunRemove_NoMatches(t *testing.T) {
	bareRepo := setupBareRepo(t)

	var err error
	out := captureStdout(t, func() {
		err = RunRemove([]string{"--all", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No worktrees matched") {
		t.Errorf("expected 'No worktrees matched', got:\n%s", out)
	}
}

func TestRunRemove_DryRunListsMergedWorktrees(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)

	var err error
	out := captureStdout(t, func() {
		err = RunRemove([]string{"--merged", "--dry-run", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "(dry run)") {
		t.Errorf("expected '(dry run)' marker, got:\n%s", out)
	}
	if !strings.Contains(out, "Would remove 1 worktree(s):") {
		t.Errorf("expected removal preview count, got:\n%s", out)
	}
	if !strings.Contains(out, "feature/merged-branch") {
		t.Errorf("expected branch name in preview, got:\n%s", out)
	}
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Errorf("dry run must not remove the worktree: %v", statErr)
	}
}

func TestRunRemove_DryRunWarnsAboutDirtyWorktrees(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	mustWriteFile(t, filepath.Join(wtPath, "untracked.txt"), "dirty\n")

	var err error
	out := captureStdout(t, func() {
		err = RunRemove([]string{"--merged", "--dry-run", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "will be LOST") {
		t.Errorf("expected dirty-worktree marker, got:\n%s", out)
	}
	if !strings.Contains(out, "Warning:") {
		t.Errorf("expected dirty-worktree warning summary, got:\n%s", out)
	}
}

// A script reads the preview to decide whether to pass --force, so a match
// the real run refuses must be marked, and the preview must say that the run
// is refused as a whole.
func TestRunRemove_DryRunSaysWhatTheRealRunRefuses(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	mustGit(t, bareRepo, "worktree", "lock", wtPath)
	lockFile := filepath.Join(bareRepo, "worktrees", "feature-merged-branch", "locked")

	var err error
	out := captureStdout(t, func() {
		err = RunRemove([]string{"--all", "--dry-run", bareRepo})
	})
	if err != nil {
		t.Fatalf("a dry run is never refused: %v", err)
	}
	for _, want := range []string{"feature/merged-branch", "(locked)", "Without --force nothing is removed", "1 worktree(s) are at risk"} {
		if !strings.Contains(out, want) {
			t.Errorf("preview missing %q:\n%s", want, out)
		}
	}

	out = captureStdout(t, func() {
		err = RunRemove([]string{"--all", "--dry-run", "--force", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "(locked)") {
		t.Errorf("preview with --force must still mark the locked worktree:\n%s", out)
	}
	if strings.Contains(out, "nothing is removed") {
		t.Errorf("with --force the real run is not refused:\n%s", out)
	}
	if _, statErr := os.Stat(lockFile); statErr != nil {
		t.Errorf("a dry run must leave the lock in place: %v", statErr)
	}
}

func TestRunRemove_RemovesMergedWorktree(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)

	var err error
	out := captureStdout(t, func() {
		err = RunRemove([]string{"--merged", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Removed:") {
		t.Errorf("expected 'Removed:' summary, got:\n%s", out)
	}
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Errorf("expected worktree %s to be removed", wtPath)
	}
}

// A lock is a deliberate "keep this" marker, so a clean locked worktree is
// held back by the at-risk gate exactly as the TUI holds it for confirmation.
func TestRunRemove_LockedWorktreeNeedsForce(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	mustGit(t, bareRepo, "worktree", "lock", "--reason", "on a removable drive", wtPath)
	lockFile := filepath.Join(bareRepo, "worktrees", "feature-merged-branch", "locked")

	err := RunRemove([]string{"--all", bareRepo})
	if err == nil {
		t.Fatal("expected the at-risk gate to refuse a locked worktree without --force")
	}
	for _, want := range []string{"locked", "--force", "feature/merged-branch"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("gate error %q missing %q", err, want)
		}
	}
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Errorf("the gate must leave the locked worktree in place: %v", statErr)
	}
	if reason, readErr := os.ReadFile(lockFile); readErr != nil || !strings.Contains(string(reason), "on a removable drive") {
		t.Errorf("the gate must leave the lock in place: reason %q, err %v", reason, readErr)
	}

	captureStdout(t, func() {
		err = RunRemove([]string{"--all", "--force", bareRepo})
	})
	if err != nil {
		t.Fatalf("--force must unlock and remove the worktree: %v", err)
	}
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Errorf("expected locked worktree %s to be unlocked and removed", wtPath)
	}
}

func TestRunRemove_DryRunLeavesLocksInPlace(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	mustGit(t, bareRepo, "worktree", "lock", "--reason", "on a removable drive", wtPath)

	captureStdout(t, func() {
		if err := RunRemove([]string{"--merged", "--dry-run", bareRepo}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	lockFile := filepath.Join(bareRepo, "worktrees", "feature-merged-branch", "locked")
	reason, err := os.ReadFile(lockFile)
	if err != nil {
		t.Fatalf("a dry run must not unlock the worktree: %v", err)
	}
	if !strings.Contains(string(reason), "on a removable drive") {
		t.Errorf("lock reason changed: %q", reason)
	}
}

func TestRunRemove_AtRiskGateCoversUnreadableStatus(t *testing.T) {
	bareRepo := setupBareRepo(t)
	// An unborn branch has no commits: `git log` fails, enrichment stops
	// before `git status`, and every risk flag stays false.
	wtPath := filepath.Join(bareRepo, "scratch")
	mustGit(t, bareRepo, "worktree", "add", "--orphan", "-b", "scratch", wtPath)
	notes := filepath.Join(wtPath, "notes.txt")
	mustWriteFile(t, notes, "precious\n")
	mustGit(t, wtPath, "add", "notes.txt")

	err := RunRemove([]string{"--all", bareRepo})
	if err == nil {
		t.Fatal("expected the at-risk gate to refuse a worktree whose status could not be read")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("gate error %q should point at --force", err)
	}
	if _, statErr := os.Stat(notes); statErr != nil {
		t.Errorf("the gate must leave staged work untouched: %v", statErr)
	}
}

func TestRunRemove_NeverRemovesCurrentDirectoryWorktree(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	t.Chdir(wtPath)

	var err error
	var out string
	stderr := captureStderr(t, func() {
		out = captureStdout(t, func() {
			err = RunRemove([]string{"--merged", bareRepo})
		})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No worktrees matched") {
		t.Errorf("the current-directory worktree must not match, got:\n%s", out)
	}
	if !strings.Contains(stderr, "current directory") {
		t.Errorf("expected a note explaining the skip on stderr, got:\n%s", stderr)
	}
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Errorf("the worktree holding the current directory must survive: %v", statErr)
	}
}

func TestRunRemove_WarnsWhenCommitDateUnavailable(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	// A worktree whose directory is gone has no reachable commit date, so the
	// stale filter must warn and skip it rather than guess.
	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatal(err)
	}

	var err error
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			err = RunRemove([]string{"--stale", "1d", "--dry-run", bareRepo})
		})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "no commit date available") {
		t.Errorf("expected missing-commit-date warning on stderr, got:\n%s", stderr)
	}
}

func TestRunRemove_ReportsFailedRemovals(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	// A read-only worktree directory makes `git worktree remove --force` fail.
	if err := os.Chmod(wtPath, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(wtPath, 0o755) })

	var err error
	out := captureStdout(t, func() {
		captureStderr(t, func() {
			err = RunRemove([]string{"--merged", bareRepo})
		})
	})
	if err == nil {
		t.Fatal("a failed removal must return an error so the exit status is non-zero")
	}
	if !strings.Contains(out, "Failed:") {
		t.Errorf("expected failure summary, got:\n%s", out)
	}
}

func TestRunRemove_SkipsProtectedWorktrees(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	mainPath := filepath.Join(bareRepo, "main")
	mustGit(t, bareRepo, "worktree", "add", mainPath, "main")

	var err error
	out := captureStdout(t, func() {
		err = RunRemove([]string{"--all", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Skipped (protected):") || !strings.Contains(out, " 1 worktree(s)") {
		t.Errorf("expected protected-skip summary, got:\n%s", out)
	}
	if _, statErr := os.Stat(mainPath); statErr != nil {
		t.Errorf("the main worktree must survive --all: %v", statErr)
	}
}

func TestRunRemove_AtRiskGateRefusesWithoutForce(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	if err := os.WriteFile(filepath.Join(wtPath, "untracked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RunRemove([]string{"--merged", bareRepo})
	if err == nil {
		t.Fatal("expected the at-risk gate to refuse without --force")
	}
	for _, want := range []string{"--force", "feature/merged-branch"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("gate error %q missing %q", err.Error(), want)
		}
	}

	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Error("the gate must leave the worktree untouched")
	}
}

func TestRunRemove_AtRiskGateForceProceeds(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	if err := os.WriteFile(filepath.Join(wtPath, "untracked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := RunRemove([]string{"--merged", "--force", bareRepo}); err != nil {
			t.Errorf("--force must proceed past the gate: %v", err)
		}
	})
	if !strings.Contains(out, "Removed") {
		t.Errorf("expected removal output, got:\n%s", out)
	}
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Error("worktree should be removed with --force")
	}
}

func TestRunRemove_DryRunExemptFromGate(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	if err := os.WriteFile(filepath.Join(wtPath, "untracked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := RunRemove([]string{"--merged", "--dry-run", bareRepo}); err != nil {
			t.Errorf("dry-run must not be gated: %v", err)
		}
	})
	if !strings.Contains(out, "dry run") {
		t.Errorf("expected dry-run output, got:\n%s", out)
	}
}
