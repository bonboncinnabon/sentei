package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

// mustGit runs a git command in dir, failing the test on error.
func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, out)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// setupBareRepo creates a bare repo with one commit on main and returns its path.
// Mirrors the e2e fixture of the same name in package cmd_test.
func setupBareRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test.git")

	mustGit(t, tmpDir, "init", "--bare", "--initial-branch=main", bareRepo)

	cloneDir := filepath.Join(tmpDir, "clone")
	mustGit(t, tmpDir, "clone", bareRepo, cloneDir)
	mustGit(t, cloneDir, "config", "user.email", "test@test.com")
	mustGit(t, cloneDir, "config", "user.name", "Test")

	mustWriteFile(t, filepath.Join(cloneDir, "README.md"), "# test\n")
	mustGit(t, cloneDir, "add", ".")
	mustGit(t, cloneDir, "commit", "-m", "initial commit")
	mustGit(t, cloneDir, "push", "origin", "main")

	return bareRepo
}

// setupBareRepoWithMergedBranch extends setupBareRepo with a feature branch that
// is merged into main and checked out in a worktree inside the bare repo.
func setupBareRepoWithMergedBranch(t *testing.T) string {
	t.Helper()
	bareRepo := setupBareRepo(t)
	tmpDir := filepath.Dir(bareRepo)
	cloneDir := filepath.Join(tmpDir, "clone2")

	mustGit(t, tmpDir, "clone", bareRepo, cloneDir)
	mustGit(t, cloneDir, "config", "user.email", "test@test.com")
	mustGit(t, cloneDir, "config", "user.name", "Test")

	mustGit(t, cloneDir, "checkout", "-b", "feature/merged-branch")
	mustWriteFile(t, filepath.Join(cloneDir, "feature.txt"), "feature\n")
	mustGit(t, cloneDir, "add", ".")
	mustGit(t, cloneDir, "commit", "-m", "feature commit")
	mustGit(t, cloneDir, "push", "origin", "feature/merged-branch")

	mustGit(t, cloneDir, "checkout", "main")
	mustGit(t, cloneDir, "merge", "feature/merged-branch")
	mustGit(t, cloneDir, "push", "origin", "main")

	mustGit(t, bareRepo, "worktree", "add", filepath.Join(bareRepo, "feature-merged-branch"), "feature/merged-branch")

	return bareRepo
}

// setupNonBareRepo creates a regular (non-bare) repo with one commit.
func setupNonBareRepo(t *testing.T) string {
	t.Helper()
	repoDir := filepath.Join(t.TempDir(), "myrepo")

	mustGit(t, filepath.Dir(repoDir), "init", "--initial-branch=main", repoDir)
	mustGit(t, repoDir, "config", "user.email", "test@test.com")
	mustGit(t, repoDir, "config", "user.name", "Test")

	mustWriteFile(t, filepath.Join(repoDir, "README.md"), "# test\n")
	mustGit(t, repoDir, "add", ".")
	mustGit(t, repoDir, "commit", "-m", "initial commit")

	return repoDir
}
