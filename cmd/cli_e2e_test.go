package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	tmpBin := t.TempDir() + "/sentei"
	build := exec.Command("go", "build", "-o", tmpBin, ".")
	build.Dir = ".."
	out, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return tmpBin
}

func TestEcosystemsCLI(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "ecosystems")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei ecosystems failed: %v\n%s", err, out)
	}

	output := string(out)
	for _, want := range []string{"Ecosystems (", "pnpm", "go", "SOURCE"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, output)
		}
	}
}

func TestUnknownCommandCLI(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "foobar")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown command")
	}

	output := string(out)
	if !strings.Contains(output, "unknown command: foobar") {
		t.Errorf("expected 'unknown command' error, got:\n%s", output)
	}
}

func TestCleanupNonInteractive_MissingMode(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "cleanup", "--non-interactive", "--force")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing --mode")
	}

	output := string(out)
	if !strings.Contains(output, "missing required flag: --mode") {
		t.Errorf("expected 'missing required flag' error, got:\n%s", output)
	}
}

func TestCleanupNonInteractive_DestructiveWithoutForce(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "cleanup", "--non-interactive")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for destructive without --force")
	}

	output := string(out)
	if !strings.Contains(output, "destructive operation requires --force") {
		t.Errorf("expected '--force required' error, got:\n%s", output)
	}
}

func TestCleanupNonInteractive_PrunesStaleWorktrees(t *testing.T) {
	bin := buildBinary(t)
	bareRepo := setupBareRepo(t)

	// Create a worktree, then delete its directory to make it prunable.
	wtPath := filepath.Join(bareRepo, "stale-wt")
	runGitCmd(t, bareRepo, "worktree", "add", "-b", "feat/stale", wtPath, "main")
	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatalf("failed to remove worktree dir: %v", err)
	}

	// Verify it's prunable.
	out := runGitCmdOutput(t, bareRepo, "worktree", "list", "--porcelain")
	if !strings.Contains(out, "prunable") {
		t.Fatalf("expected prunable worktree in porcelain output:\n%s", out)
	}

	// Run cleanup — should prune the stale worktree.
	cmd := exec.Command(bin, "cleanup", "--mode", "safe", "--non-interactive", "--force", bareRepo)
	cmd.Env = testtmp.HermeticGitEnv()
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei cleanup failed: %v\n%s", err, result)
	}

	output := string(result)
	if !strings.Contains(output, "Pruned") || !strings.Contains(output, "stale worktree") {
		t.Errorf("expected 'Pruned ... stale worktree' in output, got:\n%s", output)
	}

	// Verify it's gone from worktree list.
	afterOut := runGitCmdOutput(t, bareRepo, "worktree", "list", "--porcelain")
	if strings.Contains(afterOut, "prunable") {
		t.Errorf("stale worktree should be gone after cleanup:\n%s", afterOut)
	}
}

func runGitCmdOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = testtmp.HermeticGitEnv()
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

func TestCleanupNonInteractive_SafeMode(t *testing.T) {
	bin := buildBinary(t)

	// Create a bare repo for the test.
	repoDir := t.TempDir()
	setupGitRepo(t, repoDir)

	cmd := exec.Command(bin, "cleanup", "--mode", "safe", "--non-interactive", "--force", repoDir)
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei cleanup --mode safe --non-interactive --force failed: %v\n%s", err, out)
	}
	// Should produce some output (the cleanup ran).
	if len(out) == 0 {
		t.Error("expected non-empty output from cleanup")
	}
}

func TestCleanupNonInteractive_InvalidMode(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "cleanup", "--mode", "invalid", "--non-interactive", "--force")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for invalid mode")
	}

	output := string(out)
	if !strings.Contains(output, "invalid value for --mode") {
		t.Errorf("expected 'invalid value for --mode' error, got:\n%s", output)
	}
}

func setupGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test"},
	} {
		c := exec.Command("git", args...)
		c.Env = testtmp.HermeticGitEnv()
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
}

func TestCloneNonInteractive_MissingURL(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "clone", "--non-interactive")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing --url")
	}

	output := string(out)
	if !strings.Contains(output, "missing required flag: --url") {
		t.Errorf("expected 'missing required flag: --url' error, got:\n%s", output)
	}
}

func setupBareRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test.git")

	runGitCmd(t, tmpDir, "init", "--bare", "--initial-branch=main", bareRepo)

	cloneDir := filepath.Join(tmpDir, "clone")
	runGitCmd(t, tmpDir, "clone", bareRepo, cloneDir)
	runGitCmd(t, cloneDir, "config", "user.email", "test@test.com")
	runGitCmd(t, cloneDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(cloneDir, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	runGitCmd(t, cloneDir, "add", ".")
	runGitCmd(t, cloneDir, "commit", "-m", "initial commit")
	runGitCmd(t, cloneDir, "push", "origin", "main")

	return bareRepo
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = testtmp.HermeticGitEnv()
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, out)
	}
}

func TestCreateNonInteractive_Success(t *testing.T) {
	bin := buildBinary(t)
	bareRepo := setupBareRepo(t)

	cmd := exec.Command(bin, "create", "--branch", "feature/e2e-test", "--base", "main", "--non-interactive", bareRepo)
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei create --non-interactive failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Worktree created") {
		t.Errorf("expected 'Worktree created' in output, got:\n%s", output)
	}

	// Verify the worktree directory actually exists.
	wtPath := filepath.Join(bareRepo, "feature-e2e-test")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("expected worktree directory to exist at %s", wtPath)
	}
}

func TestCreateNonInteractive_MissingBranch(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "create", "--non-interactive")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing --branch")
	}

	output := string(out)
	if !strings.Contains(output, "missing required flag: --branch") {
		t.Errorf("expected 'missing required flag: --branch' error, got:\n%s", output)
	}
}

func TestMigrateNonInteractive_DestructiveWithoutForce(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "migrate", "--non-interactive")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for destructive without --force")
	}

	output := string(out)
	if !strings.Contains(output, "destructive operation requires --force") {
		t.Errorf("expected '--force required' error, got:\n%s", output)
	}
}

func TestMigrateNonInteractive_Success(t *testing.T) {
	bin := buildBinary(t)

	// Create a non-bare repo with a commit (migrate converts non-bare to bare).
	repoDir := filepath.Join(t.TempDir(), "myrepo")
	runGitCmd(t, t.TempDir(), "init", repoDir)
	runGitCmd(t, repoDir, "config", "user.email", "test@test.com")
	runGitCmd(t, repoDir, "config", "user.name", "Test")

	// Need an initial commit so there's a branch to detect.
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	runGitCmd(t, repoDir, "add", ".")
	runGitCmd(t, repoDir, "commit", "-m", "initial commit")

	cmd := exec.Command(bin, "migrate", "--force", "--non-interactive", repoDir)
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei migrate --force --non-interactive failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Migration complete") {
		t.Errorf("expected 'Migration complete' in output, got:\n%s", output)
	}

	// Verify .bare directory exists (confirms migration happened).
	barePath := filepath.Join(repoDir, ".bare")
	if _, err := os.Stat(barePath); os.IsNotExist(err) {
		t.Errorf("expected .bare directory at %s", barePath)
	}
}

func setupBareRepoWithMergedBranch(t *testing.T) string {
	t.Helper()
	bareRepo := setupBareRepo(t)
	tmpDir := filepath.Dir(bareRepo)
	cloneDir := filepath.Join(tmpDir, "clone2")

	runGitCmd(t, tmpDir, "clone", bareRepo, cloneDir)
	runGitCmd(t, cloneDir, "config", "user.email", "test@test.com")
	runGitCmd(t, cloneDir, "config", "user.name", "Test")

	// Create a feature branch, add a commit, push it.
	runGitCmd(t, cloneDir, "checkout", "-b", "feature/merged-branch")
	if err := os.WriteFile(filepath.Join(cloneDir, "feature.txt"), []byte("feature\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	runGitCmd(t, cloneDir, "add", ".")
	runGitCmd(t, cloneDir, "commit", "-m", "feature commit")
	runGitCmd(t, cloneDir, "push", "origin", "feature/merged-branch")

	// Merge the feature branch into main.
	runGitCmd(t, cloneDir, "checkout", "main")
	runGitCmd(t, cloneDir, "merge", "feature/merged-branch")
	runGitCmd(t, cloneDir, "push", "origin", "main")

	// Create a worktree in the bare repo for the merged branch.
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	runGitCmd(t, bareRepo, "worktree", "add", wtPath, "feature/merged-branch")

	return bareRepo
}

func TestRemoveNonInteractive_MergedBranch(t *testing.T) {
	bin := buildBinary(t)
	bareRepo := setupBareRepoWithMergedBranch(t)

	cmd := exec.Command(bin, "remove", "--merged", "--force", "--non-interactive", bareRepo)
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei remove --merged --force --non-interactive failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Removed") {
		t.Errorf("expected 'Removed' in output, got:\n%s", output)
	}
}

func TestRemoveNonInteractive_DestructiveWithoutForce(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "remove", "--merged", "--non-interactive")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for destructive without --force")
	}

	output := string(out)
	if !strings.Contains(output, "destructive operation requires --force") {
		t.Errorf("expected '--force required' error, got:\n%s", output)
	}
}

func TestRemoveNonInteractive_NoFilters(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "remove", "--force", "--non-interactive")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for no filters")
	}

	output := string(out)
	if !strings.Contains(output, "at least one filter") {
		t.Errorf("expected 'at least one filter' error, got:\n%s", output)
	}
}

func TestRemoveNonInteractive_DryRun(t *testing.T) {
	bin := buildBinary(t)
	bareRepo := setupBareRepoWithMergedBranch(t)

	cmd := exec.Command(bin, "remove", "--merged", "--dry-run", "--force", "--non-interactive", bareRepo)
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei remove --merged --dry-run failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "dry run") {
		t.Errorf("expected 'dry run' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Would remove") {
		t.Errorf("expected 'Would remove' in output, got:\n%s", output)
	}

	// Verify the worktree was NOT actually deleted.
	wtPath := filepath.Join(bareRepo, "feature-merged-branch")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree should still exist after dry-run")
	}
}

func TestIntegrationsCLI(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "integrations")
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sentei integrations failed: %v\n%s", err, out)
	}

	output := string(out)
	for _, want := range []string{"Integrations (2", "code-review-graph", "cocoindex-code", "https://github.com/"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, output)
		}
	}
}
