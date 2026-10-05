package playground

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bonboncinnabon/sentei/internal/fileutil"
)

func gitRun(dir string, args ...string) error {
	return gitRunEnv(dir, nil, args...)
}

func gitRunEnv(dir string, env []string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return nil
}

func Setup() (repoPath string, cleanup func(), err error) {
	// Unique per session: concurrent playground sessions (parallel e2e
	// agents, two terminals) must never share or destroy each other's repos.
	dir, err := os.MkdirTemp("", "sentei-playground-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating playground dir: %w", err)
	}

	cleanupFn := func() {
		_ = os.RemoveAll(dir)
	}

	repoPath = filepath.Join(dir, "repo.git")

	if err := gitRun(dir, "init", "--bare", "--initial-branch=main", repoPath); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("init bare repo: %w", err)
	}

	if err := seedInitialCommit(repoPath); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("seed commit: %w", err)
	}

	fixtures := []struct {
		name string
		fn   func(string) error
	}{
		{"main", addMainWorktree},
		{"feature/active", addCleanWorktree},
		{"feature/wip", addDirtyWorktree},
		{"experiment/abandoned", addUntrackedWorktree},
		{"hotfix/locked", addLockedWorktree},
		{"chore/old-deps", addOldWorktree},
		{"detached", addDetachedWorktree},
	}

	for _, f := range fixtures {
		if err := f.fn(repoPath); err != nil {
			cleanupFn()
			return "", nil, fmt.Errorf("creating %s: %w", f.name, err)
		}
	}

	return repoPath, cleanupFn, nil
}

func seedInitialCommit(repoPath string) error {
	dir := filepath.Dir(repoPath)
	tmpWork := filepath.Join(dir, "_seed")
	if err := gitRun(dir, "clone", repoPath, tmpWork); err != nil {
		return err
	}

	if err := gitRun(tmpWork, "config", "user.email", "test@sentei.dev"); err != nil {
		return err
	}
	if err := gitRun(tmpWork, "config", "user.name", "sentei test"); err != nil {
		return err
	}

	seedFile := filepath.Join(tmpWork, "README.md")
	if err := os.WriteFile(seedFile, []byte("# Test Repository\n\nCreated by sentei playground.\n"), 0o644); err != nil {
		return err
	}

	if err := gitRun(tmpWork, "add", "README.md"); err != nil {
		return err
	}
	if err := gitRun(tmpWork, "commit", "-m", "Initial commit"); err != nil {
		return err
	}
	if err := gitRun(tmpWork, "push", "origin", "HEAD"); err != nil {
		return err
	}

	return fileutil.RemoveAllRetry(tmpWork)
}

func worktreePath(repoPath, name string) string {
	safe := strings.ReplaceAll(name, "/", "-")
	return filepath.Join(filepath.Dir(repoPath), safe)
}

func addMainWorktree(repoPath string) error {
	wtPath := worktreePath(repoPath, "main")
	if err := gitRun(repoPath, "worktree", "add", wtPath, "main"); err != nil {
		return err
	}
	return configWorktree(wtPath)
}

func addCleanWorktree(repoPath string) error {
	wtPath := worktreePath(repoPath, "feature/active")
	if err := gitRun(repoPath, "worktree", "add", "-b", "feature/active", wtPath); err != nil {
		return err
	}

	if err := configWorktree(wtPath); err != nil {
		return err
	}

	f := filepath.Join(wtPath, "active.go")
	if err := os.WriteFile(f, []byte("package active\n"), 0o644); err != nil {
		return err
	}

	if err := gitRun(wtPath, "add", "active.go"); err != nil {
		return err
	}
	return gitRun(wtPath, "commit", "-m", "Add active feature")
}

func addDirtyWorktree(repoPath string) error {
	wtPath := worktreePath(repoPath, "feature/wip")
	if err := gitRun(repoPath, "worktree", "add", "-b", "feature/wip", wtPath); err != nil {
		return err
	}

	if err := configWorktree(wtPath); err != nil {
		return err
	}

	f := filepath.Join(wtPath, "wip.go")
	if err := os.WriteFile(f, []byte("package wip\n"), 0o644); err != nil {
		return err
	}
	if err := gitRun(wtPath, "add", "wip.go"); err != nil {
		return err
	}
	if err := gitRun(wtPath, "commit", "-m", "Start WIP feature"); err != nil {
		return err
	}

	return os.WriteFile(f, []byte("package wip\n\nfunc Draft() {}\n"), 0o644)
}

func addUntrackedWorktree(repoPath string) error {
	wtPath := worktreePath(repoPath, "experiment/abandoned")
	if err := gitRun(repoPath, "worktree", "add", "-b", "experiment/abandoned", wtPath); err != nil {
		return err
	}

	if err := configWorktree(wtPath); err != nil {
		return err
	}

	if err := gitRun(wtPath, "commit", "--allow-empty", "-m", "Start experiment"); err != nil {
		return err
	}

	f := filepath.Join(wtPath, "scratch.txt")
	return os.WriteFile(f, []byte("some scratch notes\n"), 0o644)
}

func addLockedWorktree(repoPath string) error {
	wtPath := worktreePath(repoPath, "hotfix/locked")
	if err := gitRun(repoPath, "worktree", "add", "-b", "hotfix/locked", wtPath); err != nil {
		return err
	}

	if err := configWorktree(wtPath); err != nil {
		return err
	}

	if err := gitRun(wtPath, "commit", "--allow-empty", "-m", "Hotfix in progress"); err != nil {
		return err
	}

	return gitRun(repoPath, "worktree", "lock", wtPath)
}

func addOldWorktree(repoPath string) error {
	wtPath := worktreePath(repoPath, "chore/old-deps")
	if err := gitRun(repoPath, "worktree", "add", "-b", "chore/old-deps", wtPath); err != nil {
		return err
	}

	if err := configWorktree(wtPath); err != nil {
		return err
	}

	oldDate := time.Now().AddDate(0, -4, 0).Format("2006-01-02T15:04:05")
	env := []string{
		"GIT_AUTHOR_DATE=" + oldDate,
		"GIT_COMMITTER_DATE=" + oldDate,
	}

	return gitRunEnv(wtPath, env, "commit", "--allow-empty", "-m", "Update dependencies")
}

func addDetachedWorktree(repoPath string) error {
	wtPath := filepath.Join(filepath.Dir(repoPath), "detached-head")
	return gitRun(repoPath, "worktree", "add", "--detach", wtPath)
}

func configWorktree(wtPath string) error {
	if err := gitRun(wtPath, "config", "user.email", "test@sentei.dev"); err != nil {
		return err
	}
	return gitRun(wtPath, "config", "user.name", "sentei test")
}
