package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func collectEvents(t *testing.T) *mock.EventCollector[Event] {
	t.Helper()
	return &mock.EventCollector[Event]{}
}

func TestResolveConfigPath(t *testing.T) {
	tests := []struct {
		name       string
		commonDir  string
		wantSuffix string
	}{
		{
			name:       "absolute path (bare repo)",
			commonDir:  "/repo/.bare",
			wantSuffix: "/repo/.bare/config",
		},
		{
			name:       "relative path (normal repo)",
			commonDir:  ".git",
			wantSuffix: ".git/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{Responses: map[string]mock.Response{
				"/repo:[rev-parse --git-common-dir]": {Output: tt.commonDir},
			}}

			path, err := resolveConfigPath(runner, "/repo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(path, tt.wantSuffix) {
				t.Errorf("path = %q, want suffix %q", path, tt.wantSuffix)
			}
		})
	}
}

func setupOrchestratorTest(t *testing.T) (*mock.Runner, string) {
	t.Helper()
	tmpDir := t.TempDir()
	bareDir := filepath.Join(tmpDir, ".bare")
	os.MkdirAll(bareDir, 0755)

	configData, _ := os.ReadFile(filepath.Join("testdata", "bloated.gitconfig"))
	configPath := filepath.Join(bareDir, "config")
	os.WriteFile(configPath, configData, 0644)

	runner := &mock.Runner{Responses: map[string]mock.Response{
		tmpDir + ":[rev-parse --git-common-dir]":       {Output: bareDir},
		tmpDir + ":[remote]":                           {Output: "origin"},
		tmpDir + ":[remote prune origin --dry-run]":    {Output: ""},
		tmpDir + ":[fetch --prune origin]":             {Output: ""},
		tmpDir + ":[branch -vv]":                       {Output: "  main abc123 [origin/main] latest"},
		tmpDir + ":[worktree list --porcelain]":        {Output: "worktree " + tmpDir + "\nbare\n\nworktree " + tmpDir + "/main\nHEAD abc\nbranch refs/heads/main"},
		tmpDir + ":[branch --format=%(refname:short)]": {Output: "main\nfeature/old"},
	}}

	return runner, tmpDir
}

func TestRun_SafeMode(t *testing.T) {
	runner, repoPath := setupOrchestratorTest(t)
	events := collectEvents(t)

	result := Run(runner, repoPath, Options{Mode: ModeSafe}, events.Emit)

	if result.NonWtBranchesDeleted != 0 {
		t.Error("safe mode should not delete non-worktree branches")
	}
	if result.NonWtBranchesRemaining == 0 {
		t.Error("safe mode should still count remaining non-worktree branches")
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestRun_AggressiveMode(t *testing.T) {
	runner, repoPath := setupOrchestratorTest(t)
	runner.Responses[repoPath+":[branch -d feature/old]"] = mock.Response{Output: "Deleted"}
	events := collectEvents(t)

	result := Run(runner, repoPath, Options{Mode: ModeAggressive}, events.Emit)

	if result.NonWtBranchesDeleted == 0 {
		t.Error("aggressive mode should delete non-worktree branches")
	}
}

func TestRun_ErrorContinues(t *testing.T) {
	runner, repoPath := setupOrchestratorTest(t)
	runner.Responses[repoPath+":[remote prune origin --dry-run]"] = mock.Response{Err: fmt.Errorf("network error")}
	events := collectEvents(t)

	result := Run(runner, repoPath, Options{Mode: ModeSafe}, events.Emit)

	if len(result.Errors) == 0 {
		t.Error("expected errors to be recorded")
	}
	foundPruneError := false
	for _, e := range result.Errors {
		if e.Step == "prune-refs" {
			foundPruneError = true
		}
	}
	if !foundPruneError {
		t.Error("expected prune-refs error to be recorded")
	}
	if result.ConfigDedupResult.Before == 0 {
		t.Error("config dedup should have run despite prune failure")
	}
}

func TestCountPrunable_NoPrunable(t *testing.T) {
	porcelain := "worktree /repo\nbare\n\nworktree /repo/main\nHEAD abc123\nbranch refs/heads/main\n"
	if got := countPrunable(porcelain); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestCountPrunable_OnePrunable(t *testing.T) {
	porcelain := "worktree /repo\nbare\n\nworktree /repo/stale\nHEAD def456\nbranch refs/heads/feat/stale\nprunable gitdir file points to non-existent location\n"
	if got := countPrunable(porcelain); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
}

func TestCountPrunable_MultiplePrunable(t *testing.T) {
	porcelain := "worktree /a\nHEAD a1\nprunable\n\nworktree /b\nHEAD b1\nprunable\n\nworktree /c\nHEAD c1\nbranch refs/heads/main\n"
	if got := countPrunable(porcelain); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}
