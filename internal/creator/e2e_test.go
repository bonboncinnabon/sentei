//go:build e2e

package creator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

func TestE2E_CreateWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Create a bare repo with an initial commit on main
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test-repo.git")

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("command failed: %s %v: %v", args[0], args[1:], err)
		}
	}

	// Init bare repo
	os.MkdirAll(bareRepo, 0755)
	run(bareRepo, "git", "init", "--bare")

	// Create a temporary clone to make initial commit
	cloneDir := filepath.Join(tmpDir, "clone")
	run(tmpDir, "git", "clone", bareRepo, cloneDir)
	run(cloneDir, "git", "config", "user.email", "test@test.com")
	run(cloneDir, "git", "config", "user.name", "Test")

	// Create initial commit with go.mod
	os.WriteFile(filepath.Join(cloneDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)
	os.WriteFile(filepath.Join(cloneDir, ".env"), []byte("SECRET=test\n"), 0644)
	run(cloneDir, "git", "add", ".")
	run(cloneDir, "git", "commit", "-m", "initial commit")
	run(cloneDir, "git", "push", "origin", "main")

	// Create main worktree from bare repo
	mainWT := filepath.Join(tmpDir, "main")
	run(bareRepo, "git", "worktree", "add", mainWT, "main")

	// Run creator pipeline
	runner := &git.GitRunner{}

	opts := Options{
		BranchName:     "feature/test-create",
		BaseBranch:     "main",
		RepoPath:       bareRepo,
		SourceWorktree: mainWT,
		MergeBase:      false,
		CopyEnvFiles:   true,
		Ecosystems: []config.EcosystemConfig{
			{
				Name:     "go",
				EnvFiles: []string{".env"},
			},
		},
	}

	shell := &git.DefaultShellRunner{}
	var events []progress.Event
	result := Run(runner, shell, opts, func(e progress.Event) {
		events = append(events, e)
	})

	// Verify worktree was created
	expectedPath := filepath.Join(bareRepo, "feature-test-create")
	if result.WorktreePath != expectedPath {
		t.Errorf("WorktreePath = %q, want %q", result.WorktreePath, expectedPath)
	}

	// Verify directory exists
	if _, err := os.Stat(result.WorktreePath); os.IsNotExist(err) {
		t.Fatal("worktree directory was not created")
	}

	// Verify branch exists
	out, err := runner.Run(result.WorktreePath, "branch", "--show-current")
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if out != "feature/test-create" {
		t.Errorf("branch = %q, want %q", out, "feature/test-create")
	}

	// Verify env file was copied
	envPath := filepath.Join(result.WorktreePath, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("env file not copied: %v", err)
	}
	if string(data) != "SECRET=test\n" {
		t.Errorf("env file content = %q, want %q", string(data), "SECRET=test\n")
	}

	// Verify events were emitted
	if len(events) == 0 {
		t.Error("expected events to be emitted")
	}

	// Verify no failures in Setup phase create step
	for _, phase := range result.Phases {
		if phase.Name == "Setup" {
			for _, step := range phase.Steps {
				if step.Status == progress.StepFailed && step.Name == "Create worktree" {
					t.Errorf("create worktree step failed: %v", step.Error)
				}
			}
		}
	}
}
