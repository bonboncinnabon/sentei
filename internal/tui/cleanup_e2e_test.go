package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

const testTimeout = 10 * time.Second

// setupBareRepoModel creates a Model connected to a real bare repo with worktrees loaded.
func setupBareRepoModel(t *testing.T) Model {
	t.Helper()

	tmpDir := t.TempDir()
	repoPath := createBareRepoWithWorktrees(t, tmpDir)

	runner := &git.GitRunner{}
	cfg, _ := config.LoadConfig(repoPath, config.WithRunner(runner))

	m := NewMenuModel(runner, nil, repoPath, cfg, repo.ContextBareRepo)

	// Pre-load worktrees so the menu is fully populated.
	wts, err := git.ListWorktrees(runner, repoPath)
	if err != nil {
		t.Fatalf("listing worktrees: %v", err)
	}
	wts = worktree.EnrichWorktrees(runner, wts, 10)
	var filtered []git.Worktree
	for _, wt := range wts {
		if !wt.IsBare {
			filtered = append(filtered, wt)
		}
	}
	m.remove.worktrees = filtered
	m.reindex()
	m.updateMenuHints()

	return m
}

// createBareRepoWithWorktrees creates a minimal bare repo with one worktree.
func createBareRepoWithWorktrees(t *testing.T, baseDir string) string {
	t.Helper()

	bareRepo := baseDir + "/test.git"

	runGitCmd(t, baseDir, "init", "--bare", "--initial-branch=main", bareRepo)

	cloneDir := baseDir + "/clone"
	runGitCmd(t, baseDir, "clone", bareRepo, cloneDir)
	runGitCmd(t, cloneDir, "config", "user.email", "test@test.com")
	runGitCmd(t, cloneDir, "config", "user.name", "Test")

	writeTestFile(t, cloneDir+"/README.md", "# Test\n")
	runGitCmd(t, cloneDir, "add", ".")
	runGitCmd(t, cloneDir, "commit", "-m", "initial commit")
	runGitCmd(t, cloneDir, "push", "origin", "main")

	// Create one worktree.
	wtPath := bareRepo + "/wt-0"
	runGitCmd(t, bareRepo, "worktree", "add", "-b", "feature/wt-0", wtPath, "main")
	runGitCmd(t, wtPath, "config", "user.email", "test@test.com")
	runGitCmd(t, wtPath, "config", "user.name", "Test")
	writeTestFile(t, wtPath+"/file.txt", "content\n")
	runGitCmd(t, wtPath, "add", ".")
	runGitCmd(t, wtPath, "commit", "-m", "commit on wt-0")

	return bareRepo
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = testtmp.HermeticGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, out)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

// TestE2E_CleanupConfirmUnit tests the cleanup confirmation flow using direct
// model updates (no teatest streaming) for deterministic behavior.
func TestE2E_CleanupConfirmUnit(t *testing.T) {
	m := setupBareRepoModel(t)

	// Simulate navigating to cleanup from menu: set view directly.
	m.view = cleanupConfirmView

	// Verify confirmation view renders expected content.
	view := m.viewCleanupConfirm()
	if !strings.Contains(view, "Confirm cleanup") {
		t.Errorf("expected 'Confirm cleanup' in view, got:\n%s", view)
	}
	if !strings.Contains(view, "safe") {
		t.Errorf("expected 'safe' mode in view, got:\n%s", view)
	}

	// Send ConfirmProceedMsg — should transition to cleanupResultView.
	newModel, cmd := m.Update(ConfirmProceedMsg{})
	m = newModel.(Model)
	if m.view != cleanupResultView {
		t.Errorf("expected cleanupResultView after proceed, got %d", m.view)
	}
	if cmd == nil {
		t.Error("expected a command to run cleanup")
	}
}

// TestE2E_CleanupWithModeSafePrefilled tests that SetCleanupOpts starts at confirmation.
func TestE2E_CleanupWithModeSafePrefilled(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := createBareRepoWithWorktrees(t, tmpDir)

	runner := &git.GitRunner{}
	cfg, _ := config.LoadConfig(repoPath, config.WithRunner(runner))

	m := NewMenuModel(runner, nil, repoPath, cfg, repo.ContextBareRepo)
	m.SetCleanupOpts(&cleanup.Options{Mode: cleanup.ModeSafe})

	// Model should start at cleanupConfirmView.
	if m.view != cleanupConfirmView {
		t.Errorf("expected cleanupConfirmView after SetCleanupOpts, got %d", m.view)
	}

	// Verify confirmation view shows the prefilled mode.
	view := m.viewCleanupConfirm()
	if !strings.Contains(view, "safe") {
		t.Errorf("expected 'safe' in view, got:\n%s", view)
	}
	if !strings.Contains(view, "sentei cleanup") {
		t.Errorf("expected CLI command echo in view, got:\n%s", view)
	}
}

// TestE2E_CleanupConfirmBackQuitsWhenDirectLaunch tests Esc quits when
// launched directly (not from menu).
func TestE2E_CleanupConfirmBackQuitsWhenDirectLaunch(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := createBareRepoWithWorktrees(t, tmpDir)

	runner := &git.GitRunner{}
	cfg, _ := config.LoadConfig(repoPath, config.WithRunner(runner))

	m := NewMenuModel(runner, nil, repoPath, cfg, repo.ContextBareRepo)
	m.SetCleanupOpts(&cleanup.Options{Mode: cleanup.ModeSafe})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))

	// Send Esc — should quit since we launched directly into confirmation.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	tm.WaitFinished(t, teatest.WithFinalTimeout(testTimeout))
}

// TestE2E_CleanupConfirmBackReturnsToMenu tests Esc returns to menu when
// reached from the menu (not direct launch).
func TestE2E_CleanupConfirmBackReturnsToMenu(t *testing.T) {
	m := setupBareRepoModel(t)

	// Simulate reaching confirmation from menu.
	m.view = cleanupConfirmView
	// cleanupOpts is nil (not set via SetCleanupOpts), so back goes to menu.

	newModel, _ := m.Update(ConfirmBackMsg{})
	m = newModel.(Model)
	if m.view != menuView {
		t.Errorf("expected menuView after back from menu-launched confirm, got %d", m.view)
	}
}

// TestE2E_FullCleanupFlow tests the complete flow: prefilled opts → confirm
// → cleanup executes → result view → quit. Uses FinalModel for deterministic
// state assertions rather than streaming output parsing.
func TestE2E_FullCleanupFlow(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := createBareRepoWithWorktrees(t, tmpDir)

	runner := &git.GitRunner{}
	cfg, _ := config.LoadConfig(repoPath, config.WithRunner(runner))

	m := NewMenuModel(runner, nil, repoPath, cfg, repo.ContextBareRepo)
	m.SetCleanupOpts(&cleanup.Options{Mode: cleanup.ModeSafe})

	// Test the flow synchronously through direct model updates.
	// This avoids timing issues with teatest streaming.

	// Step 1: Starts at confirmation view.
	if m.view != cleanupConfirmView {
		t.Fatalf("expected cleanupConfirmView, got %d", m.view)
	}

	// Step 2: Confirm → transitions to result view and returns a cleanup cmd.
	newModel, cmd := m.Update(ConfirmProceedMsg{})
	m = newModel.(Model)
	if m.view != cleanupResultView {
		t.Fatalf("expected cleanupResultView after proceed, got %d", m.view)
	}
	if cmd == nil {
		t.Fatal("expected a command to run cleanup")
	}

	// Step 3: Execute the cleanup command (simulates Bubble Tea's runtime).
	// The entry batch includes the breath tick; pumpCmds expands it and
	// drops animation messages.
	m = pumpCmds(m, cmd).(Model)

	// Step 4: Verify cleanup result is populated.
	if m.cleanupResult == nil {
		t.Fatal("expected cleanupResult to be set after cleanup runs")
	}

	// Step 5: Verify the result view renders.
	view := m.viewCleanupResult()
	if !strings.Contains(view, "Cleanup") {
		t.Errorf("expected 'Cleanup' in result view, got:\n%s", view)
	}
}
