package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A flag placed after the positional repo path is not parsed by the flag
// package. Accepting it silently turned `remove --all <repo> --dry-run` into a
// real removal.
func TestParseFlags_RejectArgumentsAfterRepoPath(t *testing.T) {
	tests := []struct {
		name  string
		parse func([]string) error
		args  []string
		stray string
	}{
		{
			name:  "remove",
			parse: func(args []string) error { _, err := ParseRemoveFlags(args); return err },
			args:  []string{"--all", "/repo", "--dry-run"},
			stray: "--dry-run",
		},
		{
			name:  "cleanup",
			parse: func(args []string) error { _, _, err := ParseCleanupFlags(args); return err },
			args:  []string{"--mode", "aggressive", "/repo", "--dry-run"},
			stray: "--dry-run",
		},
		{
			name:  "create",
			parse: func(args []string) error { _, err := ParseCreateFlags(args); return err },
			args:  []string{"--branch", "x", "/repo", "--base", "main"},
			stray: "--base",
		},
		{
			name:  "migrate",
			parse: func(args []string) error { _, err := ParseMigrateFlags(args); return err },
			args:  []string{"/repo", "--delete-backup"},
			stray: "--delete-backup",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parse(tt.args)
			if err == nil {
				t.Fatalf("args %v: expected an error for the argument after the repo path", tt.args)
			}
			if !strings.Contains(err.Error(), tt.stray) {
				t.Errorf("error %q should name the stray argument %q", err, tt.stray)
			}
		})
	}
}

func TestParseFlags_AcceptRepoPathLast(t *testing.T) {
	opts, err := ParseRemoveFlags([]string{"--all", "--dry-run", "/repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.RepoPath != "/repo" || !opts.DryRun {
		t.Errorf("got RepoPath=%q DryRun=%v, want /repo and true", opts.RepoPath, opts.DryRun)
	}
}

func TestRunRemove_DryRunAfterRepoPathDoesNotRemove(t *testing.T) {
	bareRepo := setupBareRepoWithMergedBranch(t)

	err := RunRemove([]string{"--merged", bareRepo, "--dry-run"})
	if err == nil {
		t.Fatal("expected an error: --dry-run after the repo path is never parsed")
	}
	if _, statErr := os.Stat(filepath.Join(bareRepo, "feature-merged-branch")); statErr != nil {
		t.Errorf("the worktree must survive a rejected command: %v", statErr)
	}
}

func TestRunCleanup_DryRunAfterRepoPathDoesNotDelete(t *testing.T) {
	bareRepo := setupBareRepo(t)
	mustGit(t, bareRepo, "branch", "extra", "main")

	err := RunCleanup([]string{"--mode", "aggressive", bareRepo, "--dry-run"})
	if err == nil {
		t.Fatal("expected an error: --dry-run after the repo path is never parsed")
	}
	mustGit(t, bareRepo, "rev-parse", "--verify", "--quiet", "refs/heads/extra")
}
