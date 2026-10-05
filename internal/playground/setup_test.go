package playground

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

func TestSetup_CreatesExpectedWorktrees(t *testing.T) {
	repoPath, cleanup, err := Setup()
	if err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer cleanup()

	wtList := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	wtList.Env = testtmp.HermeticGitEnv()
	out, err := wtList.Output()
	if err != nil {
		t.Fatalf("git worktree list error: %v", err)
	}

	porcelain := string(out)

	blocks := splitWorktreeBlocks(porcelain)
	// bare repo + 7 worktrees = 8 blocks
	if len(blocks) != 8 {
		t.Errorf("expected 8 worktree blocks, got %d\nporcelain:\n%s", len(blocks), porcelain)
	}

	expectedBranches := []string{
		"refs/heads/main",
		"refs/heads/feature/active",
		"refs/heads/feature/wip",
		"refs/heads/experiment/abandoned",
		"refs/heads/hotfix/locked",
		"refs/heads/chore/old-deps",
	}
	for _, branch := range expectedBranches {
		if !strings.Contains(porcelain, "branch "+branch) {
			t.Errorf("expected branch %q in porcelain output", branch)
		}
	}

	if !strings.Contains(porcelain, "locked") {
		t.Error("expected a locked worktree in output")
	}

	if !strings.Contains(porcelain, "detached") {
		t.Error("expected a detached worktree in output")
	}
}

func TestSetup_ConcurrentSessionsAreIsolated(t *testing.T) {
	repoA, cleanupA, err := Setup()
	if err != nil {
		t.Fatalf("first Setup() error: %v", err)
	}
	defer cleanupA()

	repoB, cleanupB, err := Setup()
	if err != nil {
		t.Fatalf("second Setup() error: %v", err)
	}
	defer cleanupB()

	if repoA == repoB {
		t.Fatalf("concurrent sessions share a directory: %s", repoA)
	}

	// Destroying session B must leave session A fully functional.
	cleanupB()
	wtListA := exec.Command("git", "-C", repoA, "worktree", "list", "--porcelain")
	wtListA.Env = testtmp.HermeticGitEnv()
	if _, err := wtListA.Output(); err != nil {
		t.Fatalf("session A broken after session B cleanup: %v", err)
	}
}

func TestSetup_CleanupRemovesDir(t *testing.T) {
	repoPath, cleanup, err := Setup()
	if err != nil {
		t.Fatalf("Setup() error: %v", err)
	}

	dir := filepath.Dir(repoPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("playground dir should exist before cleanup")
	}

	cleanup()

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("playground dir should not exist after cleanup")
	}
}

func splitWorktreeBlocks(porcelain string) []string {
	var blocks []string
	var current []string
	for _, line := range strings.Split(porcelain, "\n") {
		if line == "" {
			if len(current) > 0 {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
			}
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		blocks = append(blocks, strings.Join(current, "\n"))
	}
	return blocks
}
