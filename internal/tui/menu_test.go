package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestLoadWorktreeContext_IncludesGeneration(t *testing.T) {
	runner := &stubRunner{
		responses: map[string]stubResponse{
			"/repo worktree list --porcelain": {output: ""},
		},
	}

	var generation uint64 = 42
	cmd := loadWorktreeContext(runner, "/repo", generation)
	msg := cmd()

	ctx, ok := msg.(worktreeContextMsg)
	if !ok {
		t.Fatalf("expected worktreeContextMsg, got %T", msg)
	}
	if ctx.generation != generation {
		t.Errorf("generation = %d, want %d", ctx.generation, generation)
	}
}

func TestLoadWorktreeContext_IncludesGenerationOnError(t *testing.T) {
	runner := &stubRunner{
		responses: map[string]stubResponse{},
	}

	var generation uint64 = 7
	cmd := loadWorktreeContext(runner, "/repo", generation)
	msg := cmd()

	ctx, ok := msg.(worktreeContextMsg)
	if !ok {
		t.Fatalf("expected worktreeContextMsg, got %T", msg)
	}
	if ctx.err == nil {
		t.Fatal("expected an error")
	}
	if ctx.generation != generation {
		t.Errorf("generation = %d, want %d", ctx.generation, generation)
	}
}

func TestGlobalHandler_AppliesWorktreesOnMatchingGeneration(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)

	worktrees := []git.Worktree{
		{Path: "/repo/feat-a", Branch: "refs/heads/feat-a"},
		{Path: "/repo/feat-b", Branch: "refs/heads/feat-b"},
	}

	for _, view := range []viewState{menuView, summaryView, progressView, createSummaryView} {
		m.view = view
		m.worktreeGeneration = 5
		updated, _ := m.Update(worktreeContextMsg{
			worktrees:  worktrees,
			generation: 5,
		})
		result := updated.(Model)

		if len(result.remove.worktrees) != 2 {
			t.Errorf("view=%d: expected 2 worktrees, got %d", view, len(result.remove.worktrees))
		}
	}
}

func TestGlobalHandler_DiscardsOnGenerationMismatch(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.worktreeGeneration = 5

	existing := []git.Worktree{{Path: "/repo/existing"}}
	m.remove.worktrees = existing

	updated, _ := m.Update(worktreeContextMsg{
		worktrees:  []git.Worktree{{Path: "/repo/stale"}},
		generation: 3,
	})
	result := updated.(Model)

	if len(result.remove.worktrees) != 1 || result.remove.worktrees[0].Path != "/repo/existing" {
		t.Error("stale generation should not overwrite worktrees")
	}
}

func TestEmptyListReload_IncrementsGeneration(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = menuView
	// Init uses generation 1. Worktrees are empty (Init response hasn't arrived yet).
	// Navigate cursor to "Remove worktrees" (index 2) and enable it.
	m.menuCursor = 2
	m.menuItems[2].enabled = true

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.worktreeGeneration != 2 {
		t.Errorf("worktreeGeneration = %d, want 2 (must differ from Init's generation 1)", model.worktreeGeneration)
	}
	if cmd == nil {
		t.Fatal("expected a loadWorktreeContext cmd for empty list")
	}
}

func TestInit_BareRepo_SetsGenerationAndReturnsCmd(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	if m.worktreeGeneration != 1 {
		t.Errorf("worktreeGeneration = %d, want 1", m.worktreeGeneration)
	}
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected a Cmd from Init for bare repo, got nil")
	}
}

func TestInit_NonBareRepo_NoWorktreeLoad(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNoRepo)
	if m.worktreeGeneration != 0 {
		t.Errorf("worktreeGeneration = %d, want 0", m.worktreeGeneration)
	}
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init must still request background detection for non-bare repos")
	}
	if _, isLoad := cmd().(worktreeContextMsg); isLoad {
		t.Error("Init must not load worktree context for non-bare repos")
	}
}

func TestUpdateProgress_CleanupComplete_IncrementsGeneration(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView
	m.worktreeGeneration = 1

	updated, cmd := m.updateProgress(cleanupCompleteMsg{})
	model := updated.(Model)

	if model.worktreeGeneration != 2 {
		t.Errorf("worktreeGeneration = %d, want 2", model.worktreeGeneration)
	}
	if cmd == nil {
		t.Fatal("expected a Cmd (tea.Batch with holdOrAdvance + loadWorktreeContext)")
	}
}

func TestUpdateCreateProgress_Complete_IncrementsGeneration(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = createProgressView
	m.worktreeGeneration = 1

	updated, cmd := m.updateCreateProgress(createCompleteMsg{})
	model := updated.(Model)

	if model.worktreeGeneration != 2 {
		t.Errorf("worktreeGeneration = %d, want 2", model.worktreeGeneration)
	}
	if cmd == nil {
		t.Fatal("expected a Cmd (tea.Batch with holdOrAdvance + loadWorktreeContext)")
	}
}

func TestUpdateIntegrationProgress_Finalized_IncrementsGeneration(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = integrationListView
	m.worktreeGeneration = 1

	updated, cmd := m.updateIntegrationProgress(integrationFinalizedMsg{})
	model := updated.(Model)

	if model.worktreeGeneration != 2 {
		t.Errorf("worktreeGeneration = %d, want 2", model.worktreeGeneration)
	}
	if cmd == nil {
		t.Fatal("expected a Cmd (tea.Batch with holdOrAdvance + loadWorktreeContext)")
	}
}

func TestUpdateIntegrationProgress_Finalized_MigrateView_NoReload(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = migrateNextView
	m.worktreeGeneration = 1

	updated, _ := m.updateIntegrationProgress(integrationFinalizedMsg{})
	model := updated.(Model)

	if model.worktreeGeneration != 1 {
		t.Errorf("worktreeGeneration = %d, want 1 (no reload for migrate flow)", model.worktreeGeneration)
	}
}

func TestUpdateRepoProgress_Done_NoReload(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = repoProgressView
	m.worktreeGeneration = 1

	updated, _ := m.updateRepoProgress(repoDoneMsg{})
	model := updated.(Model)

	if model.worktreeGeneration != 1 {
		t.Errorf("worktreeGeneration = %d, want 1 (no reload for quit flow)", model.worktreeGeneration)
	}
}

func TestUpdateCleanupResult_Done_NoReload(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = cleanupResultView
	m.worktreeGeneration = 1

	updated, _ := m.updateCleanupResult(standaloneCleanupDoneMsg{result: cleanup.Result{}})
	model := updated.(Model)

	if model.worktreeGeneration != 1 {
		t.Errorf("worktreeGeneration = %d, want 1 (no reload for quit flow)", model.worktreeGeneration)
	}
}

func TestGlobalHandler_DiscardsOnError(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.worktreeGeneration = 5

	existing := []git.Worktree{{Path: "/repo/existing"}}
	m.remove.worktrees = existing

	updated, _ := m.Update(worktreeContextMsg{
		err:        fmt.Errorf("git error"),
		generation: 5,
	})
	result := updated.(Model)

	if len(result.remove.worktrees) != 1 || result.remove.worktrees[0].Path != "/repo/existing" {
		t.Error("error response should not overwrite worktrees")
	}
}

func TestE2E_RemovalToMenu_KeypressNotSwallowed(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.worktreeGeneration = 1

	// Simulate: worktrees loaded, user completed removal, now on summary view.
	m.remove.worktrees = []git.Worktree{
		{Path: "/repo/a", Branch: "refs/heads/a"},
		{Path: "/repo/b", Branch: "refs/heads/b"},
		{Path: "/repo/c", Branch: "refs/heads/c"},
	}
	m.reindex()
	m.updateMenuHints()
	m.view = summaryView

	// Simulate eager reload already completed (arrived during summary).
	m.worktreeGeneration = 2
	updated, _ := m.Update(worktreeContextMsg{
		worktrees: []git.Worktree{
			{Path: "/repo/b", Branch: "refs/heads/b"},
			{Path: "/repo/c", Branch: "refs/heads/c"},
		},
		generation: 2,
	})
	m = updated.(Model)

	// User presses Enter on summary to return to menu.
	m.view = menuView
	m.menuCursor = 0

	// First keypress: j (move down). This must NOT be swallowed.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = updated.(Model)

	if m.menuCursor == 0 {
		t.Error("first keypress after returning to menu was swallowed (cursor didn't move)")
	}
}

func TestE2E_CreationToMenu_CountUpdated(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.worktreeGeneration = 1

	// Simulate: 2 worktrees loaded initially.
	m.remove.worktrees = []git.Worktree{
		{Path: "/repo/a", Branch: "refs/heads/a"},
		{Path: "/repo/b", Branch: "refs/heads/b"},
	}
	m.reindex()
	m.updateMenuHints()

	// After creation, eager reload brings 3 worktrees.
	m.worktreeGeneration = 2
	updated, _ := m.Update(worktreeContextMsg{
		worktrees: []git.Worktree{
			{Path: "/repo/a", Branch: "refs/heads/a"},
			{Path: "/repo/b", Branch: "refs/heads/b"},
			{Path: "/repo/c", Branch: "refs/heads/c"},
		},
		generation: 2,
	})
	m = updated.(Model)

	if len(m.remove.worktrees) != 3 {
		t.Errorf("expected 3 worktrees after creation, got %d", len(m.remove.worktrees))
	}
	if m.menuItems[2].hint != "3 available" {
		t.Errorf("menu hint = %q, want %q", m.menuItems[2].hint, "3 available")
	}
}
