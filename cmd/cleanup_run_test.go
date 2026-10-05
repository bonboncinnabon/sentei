package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestParseCleanupRepoPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no args", nil, "."},
		{"flags only", []string{"--mode", "safe", "--dry-run"}, "."},
		{"positional only", []string{"/some/repo"}, "/some/repo"},
		{"flags then positional", []string{"--mode", "safe", "/some/repo"}, "/some/repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseCleanupRepoPath(tt.args); got != tt.want {
				t.Errorf("ParseCleanupRepoPath(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

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

func TestRunCleanupWithOpts_ReportsStepErrors(t *testing.T) {
	notARepo := t.TempDir()

	var err error
	out := captureStdout(t, func() {
		err = RunCleanupWithOpts(&cleanup.Options{Mode: cleanup.ModeSafe}, notARepo)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "⚠") || !strings.Contains(out, "resolve-config") {
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
