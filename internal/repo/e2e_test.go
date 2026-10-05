//go:build e2e

package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestE2E_CreateRepo(t *testing.T) {
	dir := testtmp.RobustTempDir(t)
	runner := &git.GitRunner{}
	shell := &git.DefaultShellRunner{}

	ec := &mock.EventCollector[progress.Event]{}
	opts := CreateOptions{
		Name:          "test-repo",
		Location:      dir,
		PublishGitHub: false,
	}
	result := Create(runner, shell, opts, ec.Emit)

	repoPath := filepath.Join(dir, "test-repo")

	// Verify bare structure
	if _, err := os.Stat(filepath.Join(repoPath, ".bare")); os.IsNotExist(err) {
		t.Error(".bare directory should exist")
	}

	// Verify .git pointer
	content, err := os.ReadFile(filepath.Join(repoPath, ".git"))
	if err != nil {
		t.Fatalf("reading .git pointer: %v", err)
	}
	if string(content) != "gitdir: .bare\n" {
		t.Errorf(".git pointer content = %q, want %q", string(content), "gitdir: .bare\n")
	}

	// Verify main worktree exists
	if _, err := os.Stat(filepath.Join(repoPath, "main")); os.IsNotExist(err) {
		t.Error("main worktree should exist")
	}

	// Verify README
	if _, err := os.Stat(filepath.Join(repoPath, "main", "README.md")); os.IsNotExist(err) {
		t.Error("README.md should exist in main worktree")
	}

	// No failures
	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == progress.StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}
}

func TestE2E_CloneRepo(t *testing.T) {
	// Create a source repo to clone from
	sourceDir := testtmp.RobustTempDir(t)
	runner := &git.GitRunner{}

	// Set up a minimal git repo as origin
	runner.Run(sourceDir, "init")
	runner.Run(sourceDir, "checkout", "-b", "main")
	os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("hello"), 0644)
	runner.Run(sourceDir, "add", "-A")
	runner.Run(sourceDir, "-c", "user.email=test@test.com", "-c", "user.name=Test", "commit", "-m", "init")

	// Clone it
	cloneDir := testtmp.RobustTempDir(t)
	ec := &mock.EventCollector[progress.Event]{}
	opts := CloneOptions{
		URL:      sourceDir, // local path works as URL for git clone
		Location: cloneDir,
		Name:     "cloned",
	}
	result := Clone(runner, opts, ec.Emit)

	repoPath := filepath.Join(cloneDir, "cloned")

	// Verify bare structure
	if _, err := os.Stat(filepath.Join(repoPath, ".bare")); os.IsNotExist(err) {
		t.Error(".bare directory should exist")
	}

	// Verify .git pointer
	content, err := os.ReadFile(filepath.Join(repoPath, ".git"))
	if err != nil {
		t.Fatalf("reading .git pointer: %v", err)
	}
	if string(content) != "gitdir: .bare\n" {
		t.Errorf(".git pointer content = %q", string(content))
	}

	// Verify worktree for default branch
	if result.DefaultBranch == "" {
		t.Error("DefaultBranch should be detected")
	}
	wtPath := filepath.Join(repoPath, result.DefaultBranch)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree %q should exist", wtPath)
	}
}

func TestE2E_CloneNonMainDefaultSetsUpstream(t *testing.T) {
	// Source repo whose default branch is neither main nor master.
	sourceDir := testtmp.RobustTempDir(t)
	runner := &git.GitRunner{}

	runner.Run(sourceDir, "init")
	runner.Run(sourceDir, "checkout", "-b", "production")
	os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("hello"), 0644)
	runner.Run(sourceDir, "add", "-A")
	runner.Run(sourceDir, "-c", "user.email=test@test.com", "-c", "user.name=Test", "commit", "-m", "init")

	cloneDir := testtmp.RobustTempDir(t)
	ec := &mock.EventCollector[progress.Event]{}
	opts := CloneOptions{URL: sourceDir, Location: cloneDir, Name: "cloned"}
	result := Clone(runner, opts, ec.Emit)

	for _, phase := range result.Phases {
		for _, step := range phase.Steps {
			if step.Status == progress.StepFailed {
				t.Errorf("step %q failed: %v", step.Name, step.Error)
			}
		}
	}

	if result.DefaultBranch != "production" {
		t.Errorf("DefaultBranch = %q, want %q", result.DefaultBranch, "production")
	}

	wtPath := filepath.Join(cloneDir, "cloned", "production")
	upstream, err := runner.Run(wtPath, "rev-parse", "--abbrev-ref", "@{upstream}")
	if err != nil {
		t.Fatalf("worktree has no upstream tracking configured: %v", err)
	}
	if upstream != "origin/production" {
		t.Errorf("upstream = %q, want %q", upstream, "origin/production")
	}
}

func TestE2E_MigrateRepo(t *testing.T) {
	// Create a regular git repo
	dir := testtmp.RobustTempDir(t)
	repoPath := filepath.Join(dir, "to-migrate")
	os.MkdirAll(repoPath, 0755)

	runner := &git.GitRunner{}
	shell := &git.DefaultShellRunner{}

	runner.Run(repoPath, "init")
	runner.Run(repoPath, "checkout", "-b", "main")
	os.WriteFile(filepath.Join(repoPath, "file.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(repoPath, ".env"), []byte("SECRET=val"), 0644)
	runner.Run(repoPath, "add", "file.txt")
	runner.Run(repoPath, "-c", "user.email=test@test.com", "-c", "user.name=Test", "commit", "-m", "init")

	ec := &mock.EventCollector[progress.Event]{}
	opts := MigrateOptions{RepoPath: repoPath}
	result := Migrate(runner, shell, opts, ec.Emit)

	// Verify bare structure
	if _, err := os.Stat(filepath.Join(repoPath, ".bare")); os.IsNotExist(err) {
		t.Error(".bare directory should exist")
	}

	// Verify .git is now a pointer file (not a directory)
	info, err := os.Stat(filepath.Join(repoPath, ".git"))
	if err != nil {
		t.Fatalf("stat .git: %v", err)
	}
	if info.IsDir() {
		t.Error(".git should be a file (pointer), not a directory")
	}

	// Verify root directory is clean — only .bare, .git, and worktree dir should remain
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		t.Fatalf("reading repo root: %v", err)
	}
	allowedRootEntries := map[string]bool{".bare": true, ".git": true, "main": true}
	for _, entry := range entries {
		if !allowedRootEntries[entry.Name()] {
			t.Errorf("unexpected file at repo root after migration: %s (old working files should be removed)", entry.Name())
		}
	}

	// Verify worktree has the tracked files
	wtFilePath := filepath.Join(result.WorktreePath, "file.txt")
	if _, err := os.Stat(wtFilePath); os.IsNotExist(err) {
		t.Error("file.txt should exist in worktree (checked out from git)")
	}

	// Verify backup exists
	if result.BackupPath == "" {
		t.Error("BackupPath should be set")
	}
	if _, err := os.Stat(result.BackupPath); os.IsNotExist(err) {
		t.Error("backup directory should exist")
	}

	// Verify .env was copied from backup to new worktree
	wtEnvPath := filepath.Join(result.WorktreePath, ".env")
	if _, err := os.Stat(wtEnvPath); os.IsNotExist(err) {
		t.Error(".env should be copied to new worktree from backup")
	}

	// Test backup cleanup
	err = DeleteBackup(result.BackupPath)
	if err != nil {
		t.Fatalf("DeleteBackup() error: %v", err)
	}
	if _, err := os.Stat(result.BackupPath); !os.IsNotExist(err) {
		t.Error("backup should be deleted after cleanup")
	}
}
